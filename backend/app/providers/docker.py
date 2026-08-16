import docker
from app.providers.base import ContainerProvider
from app.core.settings import settings

class DockerProvider(ContainerProvider):
    def __init__(self):
        self.client = docker.from_env()
        self.low = docker.APIClient()

    def create_instance(self, challenge_id: int, image: str, port: int):
        container = self.client.containers.run(
            image,
            detach=True,
            labels={"challenge": "true", "challenge_id": str(challenge_id)},
            ports={f"{port}/tcp": port},
        )
    
        bindings = container.attrs["NetworkSettings"]["Ports"].get(f"{port}/tcp")
        host_port = bindings[0]["HostPort"] if bindings else None
    
        return {
            "id": container.short_id,
            "name": container.name,
            "image": container.image.tags[0] if container.image.tags else None,
            "status": container.status,
            "created_at": container.attrs["Created"],
            "host_ip": settings.HOST_DOMAIN,
            "host_port": host_port,
            "challenge_id": challenge_id,
        }

    def list_instances(self):
        try:
            containers = self.client.containers.list()
        except Exception as e:
            print("Docker error:", e)
            return []
    
        return [
            {
                "id": c.short_id,
                "name": c.name,
                "image": c.image.tags[0] if c.image.tags else None,
                "status": c.status,
                "created_at": c.attrs['Created']
            }
            for c in containers
            if "challenge" in c.labels
        ]
    
    def get_instance(self, instance_id: str):
        try:
            container = self.client.containers.get(instance_id)
        except docker.errors.NotFound:
            return {"error": "Container not found"}
    
        if "challenge" not in container.labels:
            return {"error": "Container is not a challenge instance"}
    
        return {
            "id": container.short_id,
            "name": container.name,
            "image": container.image.tags[0] if container.image.tags else None,
            "status": container.status,
            "created_at": container.attrs.get('Created'),
            "provider": "docker"
        }

    def delete_instance(self, instance_id: str):
        try:
            container = self.client.containers.get(instance_id)
        except docker.errors.NotFound:
            return False

        if "challenge" in container.labels:
            container.stop()
            container.remove()
            return True
        else:
            return False











