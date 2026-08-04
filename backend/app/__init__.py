from fastapi import FastAPI
from app.api import include_routers
from app.db.migrations import migrate

def create_app():
    migrate()
    app = FastAPI()
    include_routers(app)
    return app