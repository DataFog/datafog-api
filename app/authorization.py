"""Authorization"""

import json
import os
import secrets
from typing import Optional

from constants import (
    AUTH_ENABLED_KEY,
    PASSWORD_KEY,
    USER_KEY,
    AuthTypes,
    ExceptionMessages,
)
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPBasic, HTTPBasicCredentials

AUTH_ENABLED = os.getenv(AUTH_ENABLED_KEY, "false").lower() == "true"
security = HTTPBasic() if AUTH_ENABLED else lambda: None


def get_authorization(credentials: Optional[HTTPBasicCredentials] = Depends(security)):
    """Helper function to validate user authorization"""
    if credentials is None:
        return AuthTypes.NO_AUTH.value

    if not is_valid_request(credentials):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=ExceptionMessages.UNAUTHORIZED.value,
            headers={"WWW-Authenticate": "Basic"},
        )

    return AuthTypes.HTTP_BASIC.value


def is_valid_request(credentials: HTTPBasicCredentials) -> bool:
    """Check if username and password are authorized"""
    authorized_user_bytes, password_bytes = load_valid_credentials()
    current_username_bytes = credentials.username.encode("utf8")
    current_password_bytes = credentials.password.encode("utf8")
    is_correct_username = secrets.compare_digest(
        current_username_bytes, authorized_user_bytes
    )
    is_correct_password = secrets.compare_digest(current_password_bytes, password_bytes)

    return is_correct_username and is_correct_password


def load_valid_credentials() -> tuple[bytes, bytes]:
    """read authentication configuration from env or fallback to file, throw if unsuccessful"""
    # Note: configuration must be complete in either the env or the file, no mix and match
    try:
        authorized_user_bytes = os.environ[USER_KEY].encode("utf8")
        password_bytes = os.environ[PASSWORD_KEY].encode("utf8")
    except KeyError:
        # auth config not set in env variables, attempt to read from .env file
        authorized_user_bytes, password_bytes = load_credentials_file()

    return (authorized_user_bytes, password_bytes)


def load_credentials_file() -> tuple[bytes, bytes]:
    """attempt to read authentication configuration from file, throw if unsuccessful"""

    with open(".env", "r") as auth_config:
        config_dict = json.load(auth_config)

    try:
        authorized_user_bytes = config_dict[USER_KEY].encode("utf8")
        password_bytes = config_dict[PASSWORD_KEY].encode("utf8")
    except KeyError as exc:
        # failed to find in file, throwing
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=ExceptionMessages.AUTH_USER_KEY.value,
            headers={"WWW-Authenticate": "Basic"},
        ) from exc

    return (authorized_user_bytes, password_bytes)
