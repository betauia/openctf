from fastapi import APIRouter, FastAPI, HTTPException
from app.dependencies import settings
import os

from contextlib import asynccontextmanager
from fastapi.responses import JSONResponse
from fastapi_discord import DiscordOAuthClient, RateLimited, Unauthorized, User
from fastapi_discord.exceptions import ClientSessionNotInitialized
from fastapi_discord.models import GuildPreview


auth_router = APIRouter(prefix='/auth', tags=['auth'])
ALGORITHM = "HS256"

SCOPES = ["identify", "email"]
discord = DiscordOAuthClient(
    settings.DISCORD_CLIENT_ID,
    settings.DISCORD_CLIENT_SECRET,
    "http://localhost:8000/auth/callback",
    SCOPES
)

@asynccontextmanager
async def lifespan(app: FastAPI):
    await discord.init()
    yield

# TODO: Registration using discord oauth2
@auth_router.get('/auth/login')
async def login():
    return {"url": discord.oauth_login_url}

@auth_router.get('/auth/callback')
async def callback(code: str):
    token, refresh_token = await discord.get_access_token(code)
    return {"access_token": token, "refresh_token": refresh_token}