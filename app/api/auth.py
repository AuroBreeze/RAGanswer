from __future__ import annotations

from fastapi import Header, HTTPException

from .. import registry
from ..config import get_settings


def service_auth(authorization: str = Header(default="")) -> None:
    """Authenticate callers of the internal ingestion and registry routes."""
    if not authorization.startswith("Bearer "):
        raise HTTPException(401, "缺少服务凭证")
    if authorization[7:] not in get_settings().tokens:
        raise HTTPException(403, "服务凭证无效")


def current_user(
    x_user_id: str = Header(default=""), x_user_roles: str = Header(default=""),
    x_user_factory: str | None = Header(default=None),
    x_user_workshop: str | None = Header(default=None),
) -> registry.User:
    """Read identity headers set by the trusted gateway."""
    if not x_user_id:
        raise HTTPException(401, "未登录")
    roles = [role.strip() for role in x_user_roles.split(",") if role.strip()]
    return registry.User(x_user_id, x_user_factory, x_user_workshop, roles)
