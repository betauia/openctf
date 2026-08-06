import secrets
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy import func
from sqlalchemy.orm import Session
from app.db.models.challenge import Challenge
from app.db.models.solve import Solve
from app.db.models.team import Team
from app.db.models.user import User
from app.api.auth import get_current_user
from app.dependencies import get_db

teams_router = APIRouter(prefix="/api/teams", tags=["teams"])


def _team_out(team: Team, db: Session, include_invite: bool = False) -> dict:
    members = db.query(User).filter(User.team_id == team.id).all()
    solve_counts = {
        row.user_id: row.count
        for row in db.query(Solve.user_id, func.count(Solve.id).label("count"))
            .filter(Solve.user_id.in_([m.id for m in members]))
            .group_by(Solve.user_id)
            .all()
    } if members else {}
    result = {
        "id": team.id,
        "name": team.name,
        "members": [{"id": u.id, "username": u.username, "score": u.score, "solves": solve_counts.get(u.id, 0)} for u in members],
        "score": sum(u.score for u in members),
    }
    if include_invite:
        result["invite_code"] = team.invite_code
    return result


@teams_router.get("")
def list_teams(db: Session = Depends(get_db)):
    teams = db.query(Team).all()
    return sorted([_team_out(t, db) for t in teams], key=lambda t: t["score"], reverse=True)


@teams_router.get("/my")
def my_team(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    if not user:
        raise HTTPException(401)
    if not user.team_id:
        return None
    team = db.query(Team).filter(Team.id == user.team_id).first()
    return _team_out(team, db, include_invite=True) if team else None


class CreateBody(BaseModel):
    name: str


@teams_router.post("")
def create_team(body: CreateBody, user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    if not user:
        raise HTTPException(401)
    if user.team_id:
        raise HTTPException(400, "Already in a team")
    if not (2 <= len(body.name) <= 32):
        raise HTTPException(400, "Team name must be 2–32 characters")
    if db.query(Team).filter(Team.name == body.name).first():
        raise HTTPException(400, "Team name already taken")
    team = Team(name=body.name, invite_code=secrets.token_urlsafe(6))
    db.add(team)
    db.commit()
    db.refresh(team)
    user.team_id = team.id
    db.commit()
    return _team_out(team, db, include_invite=True)


class JoinBody(BaseModel):
    invite_code: str


@teams_router.post("/join")
def join_team(body: JoinBody, user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    if not user:
        raise HTTPException(401)
    if user.team_id:
        raise HTTPException(400, "Already in a team")
    team = db.query(Team).filter(Team.invite_code == body.invite_code).first()
    if not team:
        raise HTTPException(404, "Invalid invite code")
    user.team_id = team.id
    db.commit()
    return _team_out(team, db, include_invite=True)


def _activity(team_id: int, db: Session) -> list:
    members = db.query(User).filter(User.team_id == team_id).all()
    if not members:
        return []
    member_map = {m.id: m for m in members}
    solves = (
        db.query(Solve)
        .filter(Solve.user_id.in_(list(member_map)))
        .order_by(Solve.solved_at.desc())
        .limit(50)
        .all()
    )
    challenge_map = {c.id: c for c in db.query(Challenge).filter(Challenge.id.in_({s.challenge_id for s in solves})).all()}
    return [
        {
            "username": member_map[s.user_id].username,
            "challenge": challenge_map[s.challenge_id].title,
            "category": challenge_map[s.challenge_id].category,
            "points": challenge_map[s.challenge_id].points,
            "solved_at": s.solved_at.isoformat() if s.solved_at else None,
        }
        for s in solves if s.challenge_id in challenge_map
    ]


@teams_router.get("/activity")
def team_activity(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    if not user or not user.team_id:
        return []
    return _activity(user.team_id, db)


@teams_router.get("/{team_id}/activity")
def team_activity_public(team_id: int, db: Session = Depends(get_db)):
    return _activity(team_id, db)


@teams_router.get("/members/{user_id}")
def member_profile(user_id: int, db: Session = Depends(get_db)):
    target = db.query(User).filter(User.id == user_id).first()
    if not target:
        raise HTTPException(404)
    solves = db.query(Solve).filter(Solve.user_id == user_id).order_by(Solve.solved_at.desc()).all()
    challenge_map = {c.id: c for c in db.query(Challenge).filter(Challenge.id.in_([s.challenge_id for s in solves])).all()}
    return {
        "id": target.id,
        "username": target.username,
        "score": target.score,
        "solves": [
            {
                "challenge": challenge_map[s.challenge_id].title,
                "category": challenge_map[s.challenge_id].category,
                "points": challenge_map[s.challenge_id].points,
                "solved_at": s.solved_at.isoformat() if s.solved_at else None,
            }
            for s in solves if s.challenge_id in challenge_map
        ],
    }


@teams_router.post("/leave")
def leave_team(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    if not user:
        raise HTTPException(401)
    if not user.team_id:
        raise HTTPException(400, "Not in a team")
    user.team_id = None
    db.commit()
    return {"ok": True}