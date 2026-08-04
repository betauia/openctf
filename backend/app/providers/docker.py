import docker
from app.providers.base import ContainerProvider
from app.core.settings import settings


def _is_challenge_container(container) -> bool:
    return container.labels.get("challenge") == "true"


def _serialize(container) -> dict:
    container.reload()
    host_port = None
    for bindings in container.ports.values():
        if bindings:
            host_port = bindings[0].get("HostPort")
            break
    return {
        "id": container.short_id,
        "name": container.name,
        "image": container.image.tags[0] if container.image.tags else None,
        "status": container.status,
        "created_at": container.attrs["Created"],
        "host_ip": settings.HOST_IP,
        "host_port": host_port,
        "challenge_id": container.labels.get("challenge_id"),
    }


class DockerProvider(ContainerProvider):
    def __init__(self):
        self.client = docker.from_env()

    def create_instance(self, challenge_id: int, image: str, port: int):
        container = self.client.containers.run(
            image,
            detach=True,
            labels={"challenge": "true", "challenge_id": str(challenge_id)},
            ports={f"{port}/tcp": None},
        )
        return _serialize(container)

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
