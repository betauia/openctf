from fastapi import FastAPI
from app.api import include_routers

def create_app():
    app = FastAPI()
    include_routers(app)
    return app