import docker
from app.providers.base import ContainerProvider

class DockerProvider(ContainerProvider):
    def __init__(self):
        self.client = docker.from_env()
        self.low = docker.APIClient()

    def create_instance(self, name: str, image: str, command: str = None, env_vars: dict = None):
        # This needs to be implemented in the future, for now we will just pass
        pass

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











