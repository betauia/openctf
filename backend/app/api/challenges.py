import json
import re
from pathlib import Path
from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import FileResponse
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session
from pydantic import BaseModel, ConfigDict
from app.db.models.challenge import Challenge
from app.db.models.solve import Solve
from app.db.models.team import Team
from app.db.models.user import User
from app.db.queries import team_member_ids
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


def _files(c: Challenge) -> list:
    return json.loads(c.files) if c.files else []

def _visible(db: Session):
    return db.query(Challenge).filter(Challenge.is_visible == True)

def _out(c: Challenge) -> ChallengeOut:
    return ChallengeOut(
        id=c.id, title=c.title, description=c.description,
        author=c.author, points=c.points, category=c.category,
        difficulty=c.difficulty, solves=c.solves,
        connection_info=c.connection_info, files=_files(c),
        docker_image=c.docker_image, docker_port=c.docker_port,
    )


def _check_flag(submitted: str, c: Challenge) -> bool:
    entries = json.loads(c.flags) if c.flags else []
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
            flags = re.IGNORECASE if entry.get("data") == "case_insensitive" else 0
            if re.fullmatch(entry.get("content", ""), submitted, flags):
                return True
    return False


@challenge_router.get("", response_model=list[ChallengeOut])
def list_challenges(db: Session = Depends(get_db)):
    return [_out(c) for c in _visible(db).all()]


@challenge_router.get("/{id}", response_model=ChallengeOut)
def get_challenge(id: int, db: Session = Depends(get_db)):
    c = _visible(db).filter(Challenge.id == id).first()
    if not c:
        raise HTTPException(status_code=404, detail="Challenge not found")
    return _out(c)


@challenge_router.get("/{id}/solves")
def challenge_solves(id: int, db: Session = Depends(get_db)):
    c = _visible(db).filter(Challenge.id == id).first()
    if not c:
        raise HTTPException(status_code=404)
    rows = (
        db.query(Solve, User, Team)
        .join(User, Solve.user_id == User.id)
        .outerjoin(Team, User.team_id == Team.id)
        .filter(Solve.challenge_id == id)
        .order_by(Solve.solved_at.desc())
        .all()
    )
    return [
        {
            "uid": user.id,
            "username": user.username,
            "team": team.name if team else None,
            "solved_at": solve.solved_at.isoformat() if solve.solved_at else None,
        }
        for solve, user, team in rows
    ]


@challenge_router.get("/{id}/files/{filename}")
def get_file(id: int, filename: str, db: Session = Depends(get_db)):
    c = _visible(db).filter(Challenge.id == id).first()
    if not c:
        raise HTTPException(status_code=404)
    if filename not in _files(c):
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
    c = next((ch for ch in _visible(db).all() if _check_flag(body.flag, ch)), None)
    if not c:
        return {"correct": False}

    if user:
        if user.team_id:
            already = db.query(Solve).filter(Solve.user_id.in_(team_member_ids(db, user.team_id)), Solve.challenge_id == c.id).first()
        else:
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
