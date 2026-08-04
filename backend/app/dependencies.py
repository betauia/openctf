from app.providers import get_provider
from app.db import SessionLocal

def get_instance_service():
    return get_provider()

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()