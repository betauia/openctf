import secrets
from fastapi import APIRouter, Depends, HTTPException, WebSocket, WebSocketDisconnect
from pydantic import BaseModel
from sqlalchemy import func
from sqlalchemy.orm import Session
from app.db.models.challenge import Challenge
from app.db.models.solve import Solve
from app.db.models.team import Team
from app.db.models.user import User
from app.api.auth import get_current_user
from app.dependencies import get_db

user_router = APIRouter(prefix="/api/users", tags=["users"])

class JoinBody(BaseModel):
    invite_code: str

class CreateBody(BaseModel):
    name: str


# User Profile Endpoints


# Get my user profile
@user_router.get("/me")
def get_my_profile(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    # Authenticate User (Not Implemented)
    if not user:
        raise HTTPException(status_code=401, detail="401 Unauthorized")
    return user

# Display specific user profile
@user_router.get("/{user_id}")
def get_user_profile(user_id: int, db: Session = Depends(get_db)):
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="404 Not Found: User not found")
    return user



# Teams Functionality


# Helper function to format team data to include members and their scores
def format_team(team: Team, db: Session) -> dict:
    members = db.query(User).filter(User.team_id == team.id).all()
    solve_counts = {
        row.user_id: row.count
        for row in db.query(Solve.user_id, func.count(Solve.id).label("count"))
            .filter(Solve.user_id.in_([m.id for m in members]))
            .group_by(Solve.user_id)
            .all()
    } if members else {}
    return {
        "id": team.id,
        "name": team.name,
        "members": [{"id": u.id, "username": u.username, "score": u.score, "solves": solve_counts.get(u.id, 0)} for u in members],
        "score": sum(u.score for u in members),
        "invite_code": team.invite_code
    }

# Remove this if it turns out to be depricated
# List all teams with their scores and members
@user_router.get("/teams")
def list_teams(db: Session = Depends(get_db)):
    teams = db.query(Team).all()
    return sorted([format_team(team, db) for team in teams], key=lambda t: t["score"], reverse=True)

# Get information about the current user's team
@user_router.get("/me/team")
def get_my_team(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    # Authenticate User (Not Implemented)
    if not user:
        raise HTTPException(status_code=401, detail="401 Unauthorized")
    
    # Check if the user is part of a team
    team = db.query(Team).filter(Team.id == user.team_id).first()
    if not team:
        return None
    else:
        return format_team(team, db)

# Join a team using an invite code
@user_router.put("/me/team")
def join_team(body: JoinBody, user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    # Authenticate User (Not Implemented)
    if not user:
        raise HTTPException(status_code=401, detail="401 Unauthorized")
    if user.team_id:
        raise HTTPException(status_code=400, detail="400 Bad Request: Already in a team")
    team = db.query(Team).filter(Team.invite_code == body.invite_code).first()
    if not team:
        raise HTTPException(status_code=404, detail="404 Not Found: Team not found")
    user.team_id = team.id
    db.commit()
    return format_team(team, db)


# Leave the current team
@user_router.delete("/me/team")
def leave_team(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    # Authenticate User (Not Implemented)
    if not user:
        raise HTTPException(status_code=401, detail="401 Unauthorized")
    if not user.team_id:
        raise HTTPException(status_code=400, detail="400 Bad Request: Not in a team")
    user.team_id = None
    db.commit()
    return {"detail": "Successfully left the team"}


# Create a new team
@user_router.post("/me/team")
def create_team(body: CreateBody, user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    # Authenticate User (Not Implemented)
    if not user:
        raise HTTPException(status_code=401, detail="401 Unauthorized")
    if user.team_id:
        raise HTTPException(status_code=400, detail="400 Bad Request: Already in a team")
    if not (2 <= len(body.name) <= 32):
        raise HTTPException(status_code=400, detail="400 Bad Request: Team name must be 2–32 characters")
    if db.query(Team).filter(Team.name == body.name).first():
        raise HTTPException(status_code=400, detail="400 Bad Request: Team name already taken")
    team = Team(name=body.name)
    db.add(team)
    db.commit()
    user.team_id = team.id
    db.commit()
    return format_team(team, db)

# Get the current user's team activity
@user_router.get("/me/activity")
def team_activity_private(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    # Authenticate User (Not Implemented)
    if not user:
        raise HTTPException(status_code=401, detail="401 Unauthorized")
    if not user.team_id:
        raise HTTPException(status_code=400, detail="400 Bad Request: Not in a team")
    team = db.query(Team).filter(Team.id == user.team_id).first()
    if not team:
        raise HTTPException(status_code=404, detail="404 Not Found: Team not found")
    return format_team(team, db)

# Get the activity of a specific team by team ID
@user_router.get("/teams/{team_id}/activity")
def team_activity_public(team_id: int, db: Session = Depends(get_db)):
    team = db.query(Team).filter(Team.id == team_id).first()
    if not team:
        raise HTTPException(status_code=404, detail="404 Not Found: Team not found")
    return format_team(team, db)

# WebSocket endpoint to provide real-time scoreboard updates
@user_router.websocket("/teams/ws/scoreboard")
def websocket_scoreboard(websocket: WebSocket, db: Session = Depends(get_db)):
    # Accept the WebSocket connection
    websocket.accept()

    try:
        while True:
            # Fetch all teams and their scores, ordered by score descending
            teams = db.query(Team).all()

            # Prepare the scoreboard data
            scoreboard = [
                {
                    "id": team.id,
                    "name": team.name,
                    "score": sum(user.score for user in team.users),
                    "members": [{"id": user.id, "username": user.username, "score": user.score} for user in team.users],
                }
                for team in teams
            ]

            # Send the scoreboard data to the client
            await websocket.send_json(scoreboard)

    except WebSocketDisconnect:
        print("Client disconnected from scoreboard WebSocket")



# Old vs New endpoints
# /api/teams/my                     ->  /api/users/me/team (GET)
# /api/teams (POST)                 ->  /api/users/me/team (POST)
# /api/teams/join                   ->  /api/users/me/team (PUT)
# /api/teams/leave                  ->  /api/users/me/team (DELETE)
# /api/teams/activity               ->  /api/users/me/team/activity
# /api/teams/{team_id}/activity     ->  /api/users/teams/{team_id}/activity
# /api/teams (GET)                  ->  /api/users/teams (GET)
# /api/teams/members/{user_id}      ->  /api/users/{user_id}
# For live scoreboard updates, use the WebSocket endpoint: /api/users/teams/ws/scoreboard