from fastapi import APIRouter

base_router = APIRouter()

@base_router.get('/ping')
def ping():
    return {'message': 'pong'}