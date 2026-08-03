from sqlalchemy import Column, DateTime, ForeignKey, Integer
from sqlalchemy.sql import func
from app.db import Base

class Solve(Base):
    __tablename__ = "solves"
    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    challenge_id = Column(Integer, ForeignKey("challenges.id"), nullable=False)
    solved_at = Column(DateTime, server_default=func.now())
