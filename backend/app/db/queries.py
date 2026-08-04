from sqlalchemy.orm import Session
from app.db.models.user import User


def team_member_ids(db: Session, team_id: int) -> list[int]:
    return [row.id for row in db.query(User.id).filter(User.team_id == team_id).all()]
