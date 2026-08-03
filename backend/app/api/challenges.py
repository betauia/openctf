from pathlib import Path
from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import FileResponse
from sqlalchemy.orm import Session
from pydantic import BaseModel
from app.db.models.challenge import Challenge
from app.dependencies import get_db

challenge_router = APIRouter(prefix="/api/challenges")

UPLOAD_DIR = Path("/app/uploads")


@challenge_router.get("")
def list_challenges(db: Session = Depends(get_db)):
    return db.query(Challenge).filter(Challenge.is_visible == True).all()


@challenge_router.get("/{id}")
def get_challenge(id: int, db: Session = Depends(get_db)):
    c = db.query(Challenge).filter(Challenge.id == id, Challenge.is_visible == True).first()
    if not c:
        raise HTTPException(status_code=404, detail="Challenge not found")
    return c


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
def submit_flag(body: FlagSubmit, db: Session = Depends(get_db)):
    c = db.query(Challenge).filter(
        Challenge.flag == body.flag,
        Challenge.is_visible == True,
    ).first()
    if not c:
        return {"correct": False}
    c.solves += 1
    db.commit()
    return {"correct": True, "challenge": c.title, "points": c.points}
