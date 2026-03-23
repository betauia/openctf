from sqlalchemy import create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
from app.dependencies import settings
import os

# If DATABASE_URL is set to sqlite, we need to add check_same_thread=False to the connection arguments
if settings.DATABASE_URL.startswith("sqlite"):
    engine = create_engine(settings.DATABASE_URL, connect_args={"check_same_thread": False})
else:
    engine = create_engine(settings.DATABASE_URL)

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()