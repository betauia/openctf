class InstanceService:
    def __init__(self, provider):
        self.provider = provider
    def list_instances(self):
        return self.provider.list_instances()
    def get_instance(self, instance_id: str):
        return self.provider.get_instance(instance_id)
    def delete_instance(self, instance_id: str):
        return self.provider.delete_instance(instance_id)