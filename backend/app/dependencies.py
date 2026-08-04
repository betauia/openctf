from app.providers import get_provider
from app.services.instance_services import InstanceService
from app.db import SessionLocal

def get_instance_service():
    provider = get_provider()
    return InstanceService(provider)

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()