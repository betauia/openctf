from pydantic_settings import BaseSettings

# Base settings class used to load environment variables and provide default values
class Settings(BaseSettings):
    DATABASE_URL: str = "sqlite:///./openctf.db"
    SECRET_KEY: str
    INSTANCE_PROVIDER: str = "docker"

settings = Settings()

