from pathlib import Path
from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import FileResponse
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session
from pydantic import BaseModel, ConfigDict
from app.db.models.challenge import Challenge
from app.db.models.solve import Solve
from app.db.models.user import User
from app.api.auth import get_current_user
from app.dependencies import get_db

challenge_router = APIRouter(prefix="/api/challenges")

UPLOAD_DIR = Path("/app/uploads")


class ChallengeOut(BaseModel):
    id: int
    title: str
    description: str | None
    author: str | None
    points: int
    category: str
    difficulty: str
    solves: int
    connection_info: str | None
    file_path: str | None

    model_config = ConfigDict(from_attributes=True)


@challenge_router.get("", response_model=list[ChallengeOut])
def list_challenges(db: Session = Depends(get_db)):
    return [ChallengeOut.model_validate(c) for c in db.query(Challenge).filter(Challenge.is_visible == True).all()]


@challenge_router.get("/{id}", response_model=ChallengeOut)
def get_challenge(id: int, db: Session = Depends(get_db)):
    c = db.query(Challenge).filter(Challenge.id == id, Challenge.is_visible == True).first()
    if not c:
        raise HTTPException(status_code=404, detail="Challenge not found")
    return ChallengeOut.model_validate(c)


@challenge_router.get("/{id}/file")
def get_file(id: int, db: Session = Depends(get_db)):
    c = db.query(Challenge).filter(Challenge.id == id, Challenge.is_visible == True).first()
    if not c or not c.file_path:
        raise HTTPException(status_code=404, detail="No file attached")
    path = UPLOAD_DIR / c.file_path
    if not path.exists():
        raise HTTPException(status_code=404, detail="File not found on server")
    return FileResponse(path, filename=c.file_path)


class FlagSubmit(BaseModel):
    flag: str


@challenge_router.post("/submit")
def submit_flag(
    body: FlagSubmit,
    db: Session = Depends(get_db),
    user: User | None = Depends(get_current_user),
):
    c = db.query(Challenge).filter(Challenge.flag == body.flag, Challenge.is_visible == True).first()
    if not c:
        return {"correct": False}

    if user:
        already = db.query(Solve).filter(Solve.user_id == user.id, Solve.challenge_id == c.id).first()
        if not already:
            db.add(Solve(user_id=user.id, challenge_id=c.id))
            user.score += c.points
            c.solves += 1
            try:
                db.commit()
            except IntegrityError:
                db.rollback()
    else:
        c.solves += 1
        db.commit()

    return {"correct": True, "challenge": c.title, "points": c.points}
