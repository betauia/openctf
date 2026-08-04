class ContainerProvider:
    def create_instance(self, challenge_id: int, image: str, port: int):
        raise NotImplementedError
    
    def list_instances(self):
        raise NotImplementedError

    def get_instance(self, instance_id: str):
        raise NotImplementedError

    def delete_instance(self, instance_id: str):
        raise NotImplementedError