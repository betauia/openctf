from app.api.base import base_router
from app.api.auth import auth_router
from app.api.instances import instance_router
from app.api.challenge import challenge_router
from app.api.teams import teams_router

def include_routers(app):
    app.include_router(base_router)
    app.include_router(auth_router)
    app.include_router(instance_router)
    app.include_router(challenge_router)  
    app.include_router(teams_router)  
