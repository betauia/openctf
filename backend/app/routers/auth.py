from fastapi import APIRouter, FastAPI, HTTPException
from app.dependencies import settings
import os

from contextlib import asynccontextmanager
from fastapi.responses import JSONResponse
from fastapi_discord import DiscordOAuthClient, RateLimited, Unauthorized, User
from fastapi_discord.exceptions import ClientSessionNotInitialized
from fastapi_discord.models import GuildPreview


auth_router = APIRouter(prefix='/auth', tags=['auth'])
