from sqlalchemy import Boolean, Column, Integer, String
from app.db import Base

class Challenge(Base):
    __tablename__ = "challenges"
    id = Column(Integer, primary_key=True, index=True)
    title = Column(String)
    description = Column(String)
    author = Column(String)
    points = Column(Integer)
    category = Column(String)
    difficulty = Column(String, default="Easy")
    solves = Column(Integer, default=0)
    connection_info = Column(String, nullable=True)
    flags = Column(String, nullable=True)       # JSON list; each entry: string or {type,content,data?}
    files = Column(String, nullable=True)       # JSON list of filenames under UPLOAD_DIR
    docker_image = Column(String, nullable=True)
    docker_port = Column(Integer, default=80, nullable=True)
    is_visible = Column(Boolean, default=False)
