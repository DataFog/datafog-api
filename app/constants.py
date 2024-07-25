"""Constants.py - to maintain project wide constants"""

from enum import Enum

# Define a regex pattern to encompass extended ASCII characters
VALID_INPUT_PATTERN = r"^[\x00-\xFF]+$"

# List of languages codes supported by DataFog
SUPPORTED_LANGUAGES = ["EN"]


class ResponseKeys(Enum):
    """Define API response headers as an enum"""

    TITLE = "entities"
    PII_TEXT = "text"
    START_IDX = "start"
    END_IDX = "end"
    ENTITY_TYPE = "type"
    LOOKUP_TABLE = "lookup_table"
