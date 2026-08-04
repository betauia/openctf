from app.core.settings import settings
from app.providers.docker import DockerProvider
from app.providers.kubernetes import KubernetesProvider

def get_provider():
    if settings.INSTANCE_PROVIDER == "kubernetes":
        return KubernetesProvider()
    else:
        return DockerProvider()
