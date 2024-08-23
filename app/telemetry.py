"""Collect anonymous statistics"""
import os
import requests
import uuid
import yaml

from urllib.parse import urlencode
from constants import API_VERSION_KEY, APP_NAME, BASE_TELEMETRY_URL, DEPLOYMENT_TYPE_KEY, UUID_KEY, SYSTEM_FILE_NAME, TELEMETRY_APP_KEY, FILE_PATH_LIST


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
        #send the UUID to telemetry tracking
        data_points = self.collect_telemetry()
        telemetry_url = create_telemetry_url(data_points)
        # send telemetry to url
        response = requests.get(telemetry_url)

        if response.status_code == 200:
            print("Sent telemetry successfully")
        else:
            # Handle the case where the request was not successful
            print(f"Request failed with status code {response.status_code}")

    def collect_telemetry(self):
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
    for path in FILE_PATH_LIST:
        filename = path + SYSTEM_FILE_NAME
        if os.path.exists(filename):
            try:
                with open(filename, "r") as config:
                    config_dict = yaml.safe_load(config)
                    uid = uuid.UUID(config_dict[UUID_KEY])
                    return uid
            except yaml.YAMLError:
                print(f"Warning: The file '{filename}' is not a valid YAML file.")
            except KeyError:
                print(f"No UUID key in file '{filename}'")
            except Exception as ex:
                print(f"Failed to parse UUID from {filename}, exception: {ex}")
    return None


def persist_uuid(value):
    for path in FILE_PATH_LIST:
        filename = path + SYSTEM_FILE_NAME
        new_data = {UUID_KEY : value}
        config_dict = load_uuid_yaml(filename)

        if config_dict is None:
            continue

        # Update the data with the new key-value pair
        config_dict.update(new_data)

        # Write the updated data back to the file
        try:
            with open(filename, 'w') as file:
                yaml.safe_dump(config_dict, file, default_flow_style=False)
            # Successfully wrote UUID to file, log and return
            print(f"Updated YAML data written to {filename}")
            return
        except (IOError, OSError) as e:
            print(f"Error writing to file '{filename}': {e}")


def load_uuid_yaml(filepath: str) -> dict:
    return load_system_yaml(filepath)


def load_system_yaml(filepath: str) -> dict:
    if not os.path.exists(filepath):
        try:
            # Creating file that does not exist
            with open(filepath, 'x'):
                return {}
        except PermissionError:
            print(f"Permission denied to create file: {filepath}")
        except OSError as e:
            print(f"Failed to create file {filepath}: {e}")
    else:
        try:
            # Open existing file to update
            with open(filepath, "r") as config:
                config_dict = yaml.safe_load(config)
                return config_dict
        except yaml.YAMLError:
            print(f"Warning: The file '{filepath}' is not a valid YAML file.")
        except Exception as ex:
            print(f"Failed to open config file '{filepath}', exception: {ex}")

    return None


def create_telemetry_url(parameters: dict) -> str:
    query_params_string = urlencode(parameters)
    return f"{BASE_TELEMETRY_URL}?{query_params_string}"

telemetry_instance = _Telemetry()
