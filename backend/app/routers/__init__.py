from app.routers.base import base_router
from app.routers.auth import auth_router


def include_routers(app):
    app.include_router(base_router)
    app.include_router(auth_router)
