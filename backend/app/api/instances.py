from fastapi import APIRouter, Depends, HTTPException
from app.services.instance_services import InstanceService
from app.dependencies import get_instance_service

instance_router = APIRouter(prefix="/api/instances")

@instance_router.get("/status")
def list_instances(svc: InstanceService = Depends(get_instance_service)):
    result = svc.list_instances()
    if result is None:
        raise HTTPException(status_code=404, detail="Error fetching instances")
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