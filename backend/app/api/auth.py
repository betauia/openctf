from fastapi import APIRouter, FastAPI, HTTPException


auth_router = APIRouter(prefix='/auth', tags=['auth'])
