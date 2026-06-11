import os
from datetime import datetime, timezone
from pathlib import Path

from sqlalchemy import Column, DateTime, Integer, String, create_engine
from sqlalchemy.orm import DeclarativeBase, sessionmaker

DATA_DIR = Path(os.environ.get("DATA_DIR", "/data"))
engine = create_engine(
    f"sqlite:///{DATA_DIR}/cloudsave.db",
    connect_args={"check_same_thread": False},
)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


class Base(DeclarativeBase):
    pass


class Game(Base):
    __tablename__ = "games"
    id = Column(String, primary_key=True)
    name = Column(String, nullable=False)
    created_at = Column(DateTime(timezone=True), default=lambda: datetime.now(timezone.utc))


class Save(Base):
    __tablename__ = "saves"
    id = Column(String, primary_key=True)
    game_id = Column(String, nullable=False, index=True)
    machine_name = Column(String, nullable=False)
    # uploaded_at: when the file reached the server.
    # saved_at: when the save content was actually last modified on the
    # source machine. saved_at is what determines which save is "newer".
    uploaded_at = Column(DateTime(timezone=True), default=lambda: datetime.now(timezone.utc))
    saved_at = Column(DateTime(timezone=True), nullable=True)
    file_size = Column(Integer, default=0)


def migrate() -> None:
    """Add columns that may be missing from a pre-existing database."""
    with engine.begin() as conn:
        cols = [row[1] for row in conn.exec_driver_sql("PRAGMA table_info(saves)").fetchall()]
        if "saved_at" not in cols:
            conn.exec_driver_sql("ALTER TABLE saves ADD COLUMN saved_at DATETIME")
            # Best-effort backfill for existing rows: we have no real save
            # time for them, so fall back to the upload time.
            conn.exec_driver_sql("UPDATE saves SET saved_at = uploaded_at WHERE saved_at IS NULL")


def create_tables() -> None:
    Base.metadata.create_all(bind=engine)


def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
