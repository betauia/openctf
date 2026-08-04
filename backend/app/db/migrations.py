from sqlalchemy import text
from app.db import Base, engine

def migrate():
    Base.metadata.create_all(bind=engine)
    with engine.connect() as conn:
        for col, typedef in [
            ("docker_image", "VARCHAR"),
            ("docker_port",  "INTEGER"),
            ("flags",        "TEXT"),
            ("files",        "TEXT"),
        ]:
            try:
                conn.execute(text(f"ALTER TABLE challenges ADD COLUMN {col} {typedef}"))
                conn.commit()
            except Exception:
                pass
        try:
            conn.execute(text("ALTER TABLE users ADD COLUMN team_id INTEGER"))
            conn.commit()
        except Exception:
            pass
        for stmt in [
            "UPDATE challenges SET flags = json_array(flag) WHERE flag IS NOT NULL AND flags IS NULL",
            "UPDATE challenges SET files = json_array(file_path) WHERE file_path IS NOT NULL AND files IS NULL",
        ]:
            try:
                conn.execute(text(stmt))
                conn.commit()
            except Exception:
                pass
