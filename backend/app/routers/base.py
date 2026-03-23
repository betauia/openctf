from fastapi import APIRouter

base_router = APIRouter()

@main_router.get('/ping')
def ping():
    return {'message': 'pong'}