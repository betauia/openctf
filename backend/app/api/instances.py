from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from app.db.models.challenge import Challenge
from app.services.instance_services import InstanceService
from app.dependencies import get_instance_service, get_db
from sqlalchemy.orm import Session

instance_router = APIRouter(prefix="/api/instances")


class SpawnRequest(BaseModel):
    challenge_id: int


@instance_router.post("")
def create_instance(
    body: SpawnRequest,
    svc: InstanceService = Depends(get_instance_service),
    db: Session = Depends(get_db),
):
    challenge = db.query(Challenge).filter(Challenge.id == body.challenge_id, Challenge.is_visible == True).first()
    if not challenge or not challenge.docker_image:
        raise HTTPException(status_code=404, detail="Challenge has no docker image")
    result = svc.create_instance(challenge.id, challenge.docker_image, challenge.docker_port or 80)
    if result is None:
        raise HTTPException(status_code=500, detail="Failed to spawn instance")
    return result


@instance_router.get("")
def list_instances(svc: InstanceService = Depends(get_instance_service)):
    result = svc.list_instances()
    if result is None:
        raise HTTPException(status_code=500, detail="Error fetching instances")
    return result


@instance_router.get("/{instance_id}")
def get_instance(instance_id: str, svc: InstanceService = Depends(get_instance_service)):
    result = svc.get_instance(instance_id)
    if result is None:
        raise HTTPException(status_code=404, detail="Instance not found")
    return result


@instance_router.delete("/{instance_id}")
def delete_instance(instance_id: str, svc: InstanceService = Depends(get_instance_service)):
    result = svc.delete_instance(instance_id)
    if result is None:
        raise HTTPException(status_code=404, detail="Instance not found")
    return {"detail": "Instance deleted successfully"}
