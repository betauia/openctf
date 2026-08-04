import json as _json
import re as _re
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
    files: list[str]
    docker_image: str | None
    docker_port: int | None

    model_config = ConfigDict(from_attributes=True)


def _out(c: Challenge) -> ChallengeOut:
    files = _json.loads(c.files) if c.files else []
    return ChallengeOut(
        id=c.id, title=c.title, description=c.description,
        author=c.author, points=c.points, category=c.category,
        difficulty=c.difficulty, solves=c.solves,
        connection_info=c.connection_info, files=files,
        docker_image=c.docker_image, docker_port=c.docker_port,
    )


def _check_flag(submitted: str, c: Challenge) -> bool:
    entries = _json.loads(c.flags) if c.flags else []
    for entry in entries:
        if isinstance(entry, str):
            if submitted == entry:
                return True
        elif entry.get("type") == "static":
            content = entry.get("content", "")
            if entry.get("data") == "case_insensitive":
                if submitted.lower() == content.lower():
                    return True
            elif submitted == content:
                return True
        elif entry.get("type") == "regex":
            re_flags = _re.IGNORECASE if entry.get("data") == "case_insensitive" else 0
            if _re.fullmatch(entry.get("content", ""), submitted, re_flags):
                return True
    return False


@challenge_router.get("", response_model=list[ChallengeOut])
def list_challenges(db: Session = Depends(get_db)):
    return [_out(c) for c in db.query(Challenge).filter(Challenge.is_visible == True).all()]


@challenge_router.get("/{id}", response_model=ChallengeOut)
def get_challenge(id: int, db: Session = Depends(get_db)):
    c = db.query(Challenge).filter(Challenge.id == id, Challenge.is_visible == True).first()
    if not c:
        raise HTTPException(status_code=404, detail="Challenge not found")
    return _out(c)


@challenge_router.get("/{id}/files/{filename}")
def get_file(id: int, filename: str, db: Session = Depends(get_db)):
    c = db.query(Challenge).filter(Challenge.id == id, Challenge.is_visible == True).first()
    if not c:
        raise HTTPException(status_code=404)
    files = _json.loads(c.files) if c.files else []
    if filename not in files:
        raise HTTPException(status_code=404, detail="File not in challenge")
    path = UPLOAD_DIR / filename
    if not path.exists():
        raise HTTPException(status_code=404, detail="File not found on server")
    return FileResponse(path, filename=filename)


class FlagSubmit(BaseModel):
    flag: str


@challenge_router.post("/submit")
def submit_flag(
    body: FlagSubmit,
    db: Session = Depends(get_db),
    user: User | None = Depends(get_current_user),
):
    c = next(
        (ch for ch in db.query(Challenge).filter(Challenge.is_visible == True).all() if _check_flag(body.flag, ch)),
        None,
    )
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
