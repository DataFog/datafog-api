"""Authorization"""

# Standard library imports
import os
import secrets
from typing import Optional

# Third party imports
from dotenv import load_dotenv
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPBasic, HTTPBasicCredentials

# Local imports
from constants import (
    AUTH_TYPE_KEY,
    PASSWORD_KEY,
    USER_KEY,
    AuthTypes,
    ExceptionMessages,
)

load_dotenv()


def get_required_authorization_type() -> AuthTypes:
    """Read the authorization type from the environment"""
    try:
        result = AuthTypes[os.getenv(AUTH_TYPE_KEY, "NO_AUTH").upper()]
    except KeyError:
        result = AuthTypes.NO_AUTH
    return result


ACTIVE_AUTH_TYPE = get_required_authorization_type()
AUTH_ENABLED = ACTIVE_AUTH_TYPE is not AuthTypes.NO_AUTH
security = HTTPBasic() if AUTH_ENABLED else lambda: None


def get_authorization(credentials: Optional[HTTPBasicCredentials] = Depends(security)):
    """Helper function to validate user authorization"""
    if AUTH_ENABLED and not is_valid_request(credentials):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=ExceptionMessages.UNAUTHORIZED.value,
            headers={"WWW-Authenticate": ACTIVE_AUTH_TYPE.value},
        )

    return ACTIVE_AUTH_TYPE


def is_valid_request(credentials) -> bool:
    """Call appropriate authentication function"""
    match ACTIVE_AUTH_TYPE:
        case AuthTypes.HTTP_BASIC:
            return is_valid_basic_request(credentials)
    return False


def is_valid_basic_request(credentials: HTTPBasicCredentials) -> bool:
    """For Basic Auth check if username and password are authorized"""
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

    try:
        authorized_user_bytes = os.environ[USER_KEY].encode("utf8")
        password_bytes = os.environ[PASSWORD_KEY].encode("utf8")
    except KeyError as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=ExceptionMessages.AUTH_USER_KEY.value,
            headers={"WWW-Authenticate": ACTIVE_AUTH_TYPE.value},
        ) from exc

    return (authorized_user_bytes, password_bytes)
