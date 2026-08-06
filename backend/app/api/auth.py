import base64, hashlib, hmac, os, time
from fastapi import APIRouter, Depends, HTTPException, Request, Response
from pydantic import BaseModel
from sqlalchemy.orm import Session
from app.core.settings import settings
from app.db.models.user import User
from app.db.models.solve import Solve
from app.dependencies import get_db

auth_router = APIRouter(prefix="/api/auth", tags=["auth"])


def _hash(password: str) -> str:
    salt = os.urandom(16)
    key = hashlib.pbkdf2_hmac("sha256", password.encode(), salt, 260000)
    return base64.b64encode(salt + key).decode()


def _verify(password: str, stored: str) -> bool:
    try:
        raw = base64.b64decode(stored)
        salt, key = raw[:16], raw[16:]
        return hmac.compare_digest(key, hashlib.pbkdf2_hmac("sha256", password.encode(), salt, 260000))
    except Exception:
        return False


def _make_token(user_id: int) -> str:
    payload = f"{user_id}:{int(time.time())}"
    sig = hmac.new(settings.SECRET_KEY.encode(), payload.encode(), hashlib.sha256).hexdigest()
    return base64.urlsafe_b64encode(f"{payload}:{sig}".encode()).decode().rstrip("=")


def _verify_token(token: str) -> int | None:
    try:
        data = base64.urlsafe_b64decode(token + "=" * (-len(token) % 4)).decode()
        parts = data.split(":")
        if len(parts) != 3:
            return None
        user_id_str, ts_str, sig = parts
        payload = f"{user_id_str}:{ts_str}"
        expected = hmac.new(settings.SECRET_KEY.encode(), payload.encode(), hashlib.sha256).hexdigest()
        if not hmac.compare_digest(sig, expected):
            return None
        if int(time.time()) - int(ts_str) > 86400 * 30:
            return None
        return int(user_id_str)
    except Exception:
        return None


def get_current_user(request: Request, db: Session = Depends(get_db)) -> User | None:
    token = request.cookies.get("session")
    if not token:
        return None
    user_id = _verify_token(token)
    if not user_id:
        return None
    return db.query(User).filter(User.id == user_id).first()


class RegisterBody(BaseModel):
    username: str
    email: str
    password: str


class LoginBody(BaseModel):
    username: str
    password: str


@auth_router.post("/register")
def register(body: RegisterBody, response: Response, db: Session = Depends(get_db)):
    if not (2 <= len(body.username) <= 32):
        raise HTTPException(400, "Username must be 2–32 characters")
    if len(body.password) < 8:
        raise HTTPException(400, "Password must be at least 8 characters")
    if db.query(User).filter((User.username == body.username) | (User.email == body.email)).first():
        raise HTTPException(400, "Username or email already taken")
    user = User(username=body.username, email=body.email, password_hash=_hash(body.password))
    db.add(user)
    db.commit()
    db.refresh(user)
    response.set_cookie("session", _make_token(user.id), httponly=True, max_age=86400 * 30, samesite="lax")
    return {"id": user.id, "username": user.username}


@auth_router.post("/login")
def login(body: LoginBody, response: Response, db: Session = Depends(get_db)):
    user = db.query(User).filter(User.username == body.username).first()
    if not user or not _verify(body.password, user.password_hash):
        raise HTTPException(401, "Invalid username or password")
    response.set_cookie("session", _make_token(user.id), httponly=True, max_age=86400 * 30, samesite="lax")
    return {"id": user.id, "username": user.username}


@auth_router.post("/logout")
def logout(response: Response):
    response.delete_cookie("session")
    return {"ok": True}


@auth_router.get("/me")
def me(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)): # rank stuff added by arch. CLEAN UP PLZ.
    if not user:
        raise HTTPException(401, "Not authenticated")
    rank = db.query(User).filter(User.score > user.score).count() + 1 
    return {"id": user.id, "username": user.username, "score": user.score, "is_admin": user.is_admin, "rank": rank}


@auth_router.get("/solves")
def solves(user: User | None = Depends(get_current_user), db: Session = Depends(get_db)):
    if not user:
        return []
    rows = db.query(Solve.challenge_id).filter(Solve.user_id == user.id).all()
    return [r.challenge_id for r in rows]
