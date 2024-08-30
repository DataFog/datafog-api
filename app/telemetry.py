"""Collect anonymous statistics"""
import os
import uuid
from urllib.parse import urlencode

import requests
import yaml

from constants import (
    API_VERSION_KEY,
    APP_NAME,
    BASE_TELEMETRY_URL,
    DEPLOYMENT_TYPE_KEY,
    FILE_PATH_LIST,
    SYSTEM_FILE_NAME,
    TELEMETRY_APP_KEY,
    UUID_KEY,
)


TELEMETRY_INSTANCE = None


class _Telemetry:
    """Controller to manage telemetry interactions"""

    def __init__(self):
        instance_uuid = load_uuid()
        if instance_uuid is None:
            # UUID not set, create a new one and set it
            instance_uuid = uuid.uuid4()
            persist_uuid(str(instance_uuid))
        self.uuid = instance_uuid


    def report_basic_telemetry(self):
        # send the UUID to telemetry tracking
        data_points = self.collect_telemetry()
        telemetry_url = create_telemetry_url(data_points)
        # send telemetry to url
        try:
            response = requests.get(telemetry_url, timeout=3)

            if response.status_code == 200:
                print("Sent telemetry successfully")
            else:
                # Handle the case where the request was not successful
                print(f"Request failed with status code {response.status_code}")
        except requests.exceptions.Timeout:
            print("Telemetry request timed out")


    def collect_telemetry(self) -> dict:
        data = {}
        data[TELEMETRY_APP_KEY] = APP_NAME
        data[UUID_KEY] = self.uuid
        deployment_type = os.getenv(DEPLOYMENT_TYPE_KEY)
        if deployment_type is not None:
            data[DEPLOYMENT_TYPE_KEY] = deployment_type
        api_version = os.getenv(API_VERSION_KEY)
        if api_version is not None:
            data[API_VERSION_KEY] = api_version

        return data


def load_uuid() -> uuid.UUID:
    """read uuid from datafog config files"""
    for config_dict, filename in config_generator(False):
        try:
            uid = uuid.UUID(config_dict[UUID_KEY])
            return uid
        except KeyError:
            print(f"No UUID key in file '{filename}'")
        except ValueError as ve:
            print(f"Malformed UUID in file '{filename}', {ve}")

    return None


def persist_uuid(value):
    new_data = {UUID_KEY : value}
    for config_dict, filename in config_generator(True):
        # Update the data with the new key-value pair
        config_dict.update(new_data)

        # Write the updated data back to the file
        try:
            with open(filename, 'w', encoding="utf-8") as file:
                yaml.safe_dump(config_dict, file, default_flow_style=False)
            # Successfully wrote UUID to file, log and return
            print(f"Updated YAML data written to {filename}")
            return
        except (IOError, OSError) as e:
            print(f"Error writing to file '{filename}': {e}")


def load_system_yaml(filepath: str, create_new: bool) -> dict:
    if os.path.exists(filepath):
        try:
            # Open existing file to update
            with open(filepath, "r", encoding="utf-8") as config:
                config_dict = yaml.safe_load(config)
                return config_dict
        except yaml.YAMLError:
            print(f"Warning: The file '{filepath}' is not a valid YAML file.")
        except Exception as ex:
            print(f"Failed to open config file '{filepath}', exception: {ex}")
    elif create_new:
        try:
            # Creating file that does not exist
            # Extract the directory path from the file path
            directory = os.path.dirname(filepath)

            # Create the directory if it doesn't exist
            if not os.path.exists(directory):
                os.makedirs(directory)
            with open(filepath, 'x', encoding="utf-8"):
                return {}
        except PermissionError:
            print(f"Permission denied to create file: {filepath}")
        except OSError as e:
            print(f"Failed to create file {filepath}: {e}")

    return None


def create_telemetry_url(parameters: dict) -> str:
    if not parameters:
        return BASE_TELEMETRY_URL
    query_params_string = urlencode(parameters)
    return f"{BASE_TELEMETRY_URL}?{query_params_string}"


def config_generator(create_new: bool):
    for path in FILE_PATH_LIST:
        expanded_path = os.path.expanduser(path)
        filepath = f"{expanded_path}{SYSTEM_FILE_NAME}"

        config_dict = load_system_yaml(filepath, create_new)

        if config_dict is not None:
            yield (config_dict, filepath)


def get_telemetry_instance() -> _Telemetry:
    global TELEMETRY_INSTANCE
    if TELEMETRY_INSTANCE is None:
        TELEMETRY_INSTANCE = _Telemetry()
    return TELEMETRY_INSTANCE
