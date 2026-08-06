from fastapi import FastAPI
from app.db import init_db
from app.api import include_routers

def create_app():
    app = FastAPI()
    include_routers(app)
    init_db()
    return app