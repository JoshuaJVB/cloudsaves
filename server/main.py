import os
import shutil
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import List

from database import Game, Save, create_tables, get_db, migrate
from fastapi import Depends, FastAPI, File, Form, Header, HTTPException, UploadFile
from fastapi.responses import FileResponse
from pydantic import BaseModel
from sqlalchemy.orm import Session

DATA_DIR = Path(os.environ.get("DATA_DIR", "/data"))
SAVES_DIR = DATA_DIR / "saves"
API_KEY = os.environ.get("API_KEY", "changeme")
MAX_SAVES = 5

app = FastAPI(title="CloudSave")


@app.on_event("startup")
def startup() -> None:
    SAVES_DIR.mkdir(parents=True, exist_ok=True)
    create_tables()
    migrate()


def _parse_saved_at(raw: str | None) -> datetime | None:
    if not raw:
        return None
    try:
        dt = datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def require_key(x_api_key: str = Header(...)) -> None:
    if x_api_key != API_KEY:
        raise HTTPException(status_code=401, detail="Invalid API key")


Auth = Depends(require_key)


class GameCreate(BaseModel):
    id: str
    name: str


class GameOut(BaseModel):
    id: str
    name: str
    created_at: datetime
    model_config = {"from_attributes": True}


class SaveOut(BaseModel):
    id: str
    game_id: str
    machine_name: str
    uploaded_at: datetime
    saved_at: datetime | None
    file_size: int
    model_config = {"from_attributes": True}


@app.get("/games", response_model=List[GameOut], dependencies=[Auth])
def list_games(db: Session = Depends(get_db)):
    return db.query(Game).order_by(Game.name).all()


@app.post("/games", response_model=GameOut, dependencies=[Auth])
def register_game(body: GameCreate, db: Session = Depends(get_db)):
    existing = db.query(Game).filter(Game.id == body.id).first()
    if existing:
        return existing
    game = Game(id=body.id, name=body.name)
    db.add(game)
    db.commit()
    db.refresh(game)
    return game


@app.get("/games/{game_id}/saves", response_model=List[SaveOut], dependencies=[Auth])
def list_saves(game_id: str, db: Session = Depends(get_db)):
    if not db.query(Game).filter(Game.id == game_id).first():
        raise HTTPException(404, "Game not found")
    return (
        db.query(Save)
        .filter(Save.game_id == game_id)
        .order_by(Save.saved_at.desc())
        .all()
    )


@app.post("/games/{game_id}/saves", response_model=SaveOut, dependencies=[Auth])
async def upload_save(
    game_id: str,
    machine_name: str = Form(...),
    saved_at: str = Form(None),
    file: UploadFile = File(...),
    db: Session = Depends(get_db),
):
    if not db.query(Game).filter(Game.id == game_id).first():
        raise HTTPException(404, "Game not found")

    now = datetime.now(timezone.utc)
    save_id = str(uuid.uuid4())
    game_dir = SAVES_DIR / game_id
    game_dir.mkdir(exist_ok=True)
    dest = game_dir / f"{save_id}.zip"

    with open(dest, "wb") as f:
        shutil.copyfileobj(file.file, f)

    save = Save(
        id=save_id,
        game_id=game_id,
        machine_name=machine_name,
        uploaded_at=now,
        saved_at=_parse_saved_at(saved_at) or now,
        file_size=dest.stat().st_size,
    )
    db.add(save)
    db.commit()
    db.refresh(save)

    _prune(game_id, game_dir, db)
    return save


def _prune(game_id: str, game_dir: Path, db: Session) -> None:
    saves = (
        db.query(Save)
        .filter(Save.game_id == game_id)
        .order_by(Save.uploaded_at.desc())
        .all()
    )
    for old in saves[MAX_SAVES:]:
        p = game_dir / f"{old.id}.zip"
        if p.exists():
            p.unlink()
        db.delete(old)
    db.commit()


@app.get("/games/{game_id}/saves/latest", dependencies=[Auth])
def download_latest(game_id: str, db: Session = Depends(get_db)):
    save = (
        db.query(Save)
        .filter(Save.game_id == game_id)
        .order_by(Save.saved_at.desc())
        .first()
    )
    if not save:
        raise HTTPException(404, "No saves found")
    return _file_response(game_id, save)


@app.get("/games/{game_id}/saves/{save_id}", dependencies=[Auth])
def download_save(game_id: str, save_id: str, db: Session = Depends(get_db)):
    save = (
        db.query(Save)
        .filter(Save.game_id == game_id, Save.id == save_id)
        .first()
    )
    if not save:
        raise HTTPException(404, "Save not found")
    return _file_response(game_id, save)


def _file_response(game_id: str, save: Save) -> FileResponse:
    path = SAVES_DIR / game_id / f"{save.id}.zip"
    ts = save.uploaded_at.strftime("%Y%m%d-%H%M%S")
    return FileResponse(
        str(path),
        filename=f"{game_id}-{ts}.zip",
        media_type="application/zip",
    )
