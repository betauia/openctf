from sqlalchemy import Boolean, Column, Integer, String, ForeignKey
from sqlalchemy.orm import relationship
from app.db.database import Base

class Challenge(Base):
    __tablename__ = "challenges"
    id = Column(Integer, primary_key=True, index=True)
    title = Column(String)
    description = Column(String)
    author = Column(String)
    flag = Column(String)
    points = Column(Integer)
    category = Column(String)
    is_visible = Column(Boolean, default=False)
