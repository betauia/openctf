from fastapi import FastAPI
from app.api import include_routers
from app.db import Base, engine

def create_app():
    Base.metadata.create_all(bind=engine)
    app = FastAPI()
    include_routers(app)
    return app