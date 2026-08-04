import docker
from app.providers.base import ContainerProvider

def _is_challenge_container(container) -> bool:
    return container.labels.get("challenge") == "true"


def _serialize(container) -> dict:
    return {
        "id": container.short_id,
        "name": container.name,
        "image": container.image.tags[0] if container.image.tags else None,
        "status": container.status,
        "created_at": container.attrs['Created']
    }


class DockerProvider(ContainerProvider):
    def __init__(self):
        self.client = docker.from_env()

    def create_instance(self, name: str, image: str, command: str = None, env_vars: dict = None):
        # This needs to be implemented in the future, for now we will just pass
        pass

    def list_instances(self):
        try:
            containers = self.client.containers.list()
        except Exception as e:
            print("Docker error:", e)
            return []

        return [_serialize(c) for c in containers if _is_challenge_container(c)]

    def get_instance(self, instance_id: str):
        try:
            container = self.client.containers.get(instance_id)
        except docker.errors.NotFound:
            return None

        if not _is_challenge_container(container):
            return None
        return _serialize(container)

    def delete_instance(self, instance_id: str):
        try:
            container = self.client.containers.get(instance_id)
        except docker.errors.NotFound:
            return False

        if not _is_challenge_container(container):
            return False
        container.stop()
        container.remove()
        return True











