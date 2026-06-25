from fastapi import Request, HTTPException, Depends, APIRouter
import docker
from pydantic import BaseModel
import yaml
from pathlib import Path

instance_router = APIRouter()
client = docker.from_env()

# Limit to localhost





# Create Docker Instance


# Delete Docker Instance
@instance_router.delete("/instances/{container_id}")
def delete_instance(container_id: str):
    try:
        container = client.containers.get(container_id)
        if container.labels.get("challenge") == "true":
            container.stop()
            container.remove()
            return {"detail": "Instance deleted successfully"}
        else:
            raise HTTPException(status_code=404, detail="Instance not found")
    except docker.errors.NotFound:
        raise HTTPException(status_code=404, detail="Instance not found")
    except docker.errors.DockerException as e:
        raise HTTPException(status_code=500, detail="Error occurred while deleting instance")

# Get all running Containers
@instance_router.get("/instances/status")
def get_challenge_instances():
    try:
        containers = client.containers.list()
    except docker.errors.DockerException as e:
        raise HTTPException(status_code=500, detail="Error occurred while fetching instances")
    
    result = []
    for c in containers:
        if c.labels.get("challenge") == "true":
            result.append({
                "id": c.short_id,
                "name": c.name,
                "image": c.image.tags[0] if c.image.tags else None,
                "status": c.status,
                "created_at": c.attrs['Created']
            })
    return result

# Get specific container
@instance_router.get("/instances/{container_id}")
def get_instance(container_id: str):
    try:
        container = client.containers.get(container_id)
        if container.labels.get("challenge") == "true":
            return {
                "id": container.short_id,
                "name": container.name,
                "image": container.image.tags[0] if container.image.tags else None,
                "status": container.status,
                "created_at": container.attrs['Created']
            }
        else:
            raise HTTPException(status_code=404, detail="Instance not found")
    except docker.errors.NotFound:
        raise HTTPException(status_code=404, detail="Instance not found")
    except docker.errors.DockerException as e:
        raise HTTPException(status_code=500, detail="Error occurred while fetching instance")