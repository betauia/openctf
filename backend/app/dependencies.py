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

def authenticate_user(token: str = Depends(oauth2_scheme), db: Session = Depends(get_db)):
    user = get_current_user(token, db)
    if not user:
        raise HTTPException(status_code=401, detail="Invalid authentication credentials")
    return user