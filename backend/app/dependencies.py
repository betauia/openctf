from app.providers import get_provider
from app.services.instance_services import InstanceService

def get_instance_service():
    provider = get_provider()
    return InstanceService(provider)