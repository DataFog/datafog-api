"""Constants.py - to maintain project wide constants"""

from enum import Enum

# Define a regex pattern to encompass extended ASCII characters
VALID_INPUT_PATTERN = r"^[\x00-\xFF]+$"

# List of languages codes supported by DataFog
SUPPORTED_LANGUAGES = ["EN"]

AUTH_ENABLED_KEY = "DATAFOG_AUTH_ENABLED"
USER_KEY = "DATAFOG_AUTH_USER"
PASSWORD_KEY = "DATAFOG_PASSWORD"


class ResponseKeys(Enum):
    """Define API response headers as an enum"""

    TITLE = "entities"
    PII_TEXT = "text"
    START_IDX = "start"
    END_IDX = "end"
    ENTITY_TYPE = "type"
    LOOKUP_TABLE = "lookup_table"


class AuthTypes(Enum):
    """Authentication Types"""

    HTTP_BASIC = "http_basic"
    NO_AUTH = "no_auth"


class ExceptionMessages(Enum):
    """Error messages returned by exceptions"""

    AUTH_USER_KEY = "Authorization configuration is not complete, please add authorized Users"
    AUTH_PASS_KEY = "Authorization configuration is not complete, please add authorized Users"
    INVALID_CHARACTER = "string contains unsupported characters beyond the Extended ASCII set"
    UNAUTHORIZED = "Incorrect username or password"
    UNSUPPORTED_LANGUAGE = "Unsupported language request, please try a language listed in the DataFog docs"
