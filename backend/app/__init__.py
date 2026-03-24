from fastapi import FastAPI
from app.routers import include_routers

def create_app():
    app = FastAPI()
    include_routers(app)
    return app