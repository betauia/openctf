from sqlalchemy import Boolean, Column, Integer, String, ForeignKey
from app.db import Base

class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True, index=True)
    username = Column(String, unique=True, index=True)
    email = Column(String, unique=True, index=True)
    password_hash = Column(String)
    score = Column(Integer, default=0)
    is_admin = Column(Boolean, default=False)
    team_id = Column(Integer, ForeignKey("teams.id"), nullable=True)
