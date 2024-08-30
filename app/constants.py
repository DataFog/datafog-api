"""Constants.py - to maintain project wide constants"""

from enum import Enum

# Define a regex pattern to encompass extended ASCII characters
VALID_INPUT_PATTERN = r"^[\x00-\xFF]+$"

# List of languages codes supported by DataFog
SUPPORTED_LANGUAGES = ["EN"]

# Authorization Constants
AUTH_TYPE_KEY = "DATAFOG_AUTH_TYPE"
USER_KEY = "DATAFOG_AUTH_USER"
PASSWORD_KEY = "DATAFOG_PASSWORD"

# Telemetry Constants
API_VERSION_KEY = "DATAFOG_API_VERSION"
APP_NAME = "datafog-api"
BASE_TELEMETRY_URL = "https://www.datafog.ai/usage"
DEPLOYMENT_TYPE_KEY = "DATAFOG_DEPLOYMENT_TYPE"
SYSTEM_FILE_NAME = "api.system.yaml"
TELEMETRY_APP_KEY = "app"
UUID_KEY = "DATAFOG_UUID"
FILE_PATH_LIST = [
    "~/.datafog/",
    "./datafog/",
    "/var/tmp/datafog/",
    "/tmp/"
]


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
    INVALID_CHAR = "string contains unsupported characters beyond the Extended ASCII set"
    UNAUTHORIZED = "Incorrect username or password"
    UNSUPPORTED_LANG = "Unsupported language, please try a language listed in the DataFog docs"
