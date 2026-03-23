#######################################################################################
# This file contains shared dependencies and data models                              #
#######################################################################################
from pydantic import BaseSettings

# Base settings class used to load environment variables and provide default values
class Settings(BaseSettings):
    DATABASE_URL: str = "sqlite:///./openctf.db"
    SECRET_KEY: str
    DISCORD_CLIENT_ID: str
    DISCORD_CLIENT_SECRET: str

settings = Settings()

