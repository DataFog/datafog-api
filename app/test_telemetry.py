'''Unit Tests for the Telemetry Module'''

# Standard library imports
from unittest.mock import ANY, patch, mock_open
from uuid import UUID

# Third party imports
from requests import Response
from requests.exceptions import Timeout
from yaml import YAMLError

# Local imports
from telemetry import (
    _Telemetry,
    create_telemetry_url,
    config_generator,
    get_telemetry_instance,
    load_system_yaml,
    load_uuid,
    persist_uuid,
)
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

TEST_API_VERSION = "1.0.0"
TEST_DEPLOY_TYPE = "test_deploy"
TEST_PATH = '/some/path'
TEST_URL = 'https://some.site.com'
TEST_UUID = "25dbae32-bb9e-49cb-844c-21ca842491e2"


def test_create_telemetry_url():
    params1 = {}
    params2 = { TELEMETRY_APP_KEY : "datafog-api",
                DEPLOYMENT_TYPE_KEY: "test" }
    result1 = create_telemetry_url(params1)
    result2 = create_telemetry_url(params2)
    correct2 = f"{BASE_TELEMETRY_URL}?{TELEMETRY_APP_KEY}=datafog-api&{DEPLOYMENT_TYPE_KEY}=test"

    assert result1 == BASE_TELEMETRY_URL, "url incorrect from empty parameters"
    assert result2 == correct2, "url incorrect from 2 parameters"

@patch("os.path.exists")
@patch('builtins.open', new_callable=mock_open, read_data=f'{UUID_KEY}: {TEST_UUID}')
def test_load_system_yaml_exist(mock_open_func, mock_exists):
    mock_exists.return_value = True
    result = load_system_yaml(TEST_PATH, False)

    mock_open_func.assert_called_once_with(TEST_PATH, 'r', encoding='utf-8')
    assert result[UUID_KEY] == TEST_UUID

@patch("os.path.exists")
@patch("os.makedirs")
@patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_not_exist(mock_open_func, mock_make_dir, mock_exists):
    mock_exists.return_value = False
    result = load_system_yaml(TEST_PATH, True)

    mock_make_dir.assert_called_once()
    mock_open_func.assert_called_once_with(TEST_PATH, 'x', encoding='utf-8')
    assert result == {}

@patch("os.path.exists")
@patch("os.makedirs")
# @patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_permission_error_make_dir(mock_make_dir, mock_exists):
    mock_exists.return_value = False
    # Set up the mock to raise a PermissionError when open is called
    mock_make_dir.side_effect = PermissionError("Permission denied")
    result = load_system_yaml(TEST_PATH, True)

    mock_make_dir.assert_called_once()
    # mock_open_func.assert_called_once_with(dummy_path, 'x', encoding='utf-8')
    assert result is None


@patch("os.path.exists")
@patch("os.makedirs")
@patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_permission_error(mock_open_func, mock_make_dir, mock_exists):
    mock_exists.return_value = False
    dummy_path = '/some/path'
    # Set up the mock to raise a PermissionError when open is called
    mock_open_func.side_effect = PermissionError("Permission denied")
    result = load_system_yaml(TEST_PATH, True)

    mock_make_dir.assert_called_once()
    mock_open_func.assert_called_once_with(TEST_PATH, 'x', encoding='utf-8')
    assert result is None

@patch("os.path.exists")
@patch("os.makedirs")
# @patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_os_error_make_dir(mock_make_dir, mock_exists):
    mock_exists.return_value = False
    dummy_path = '/some/path'
    # Set up the mock to raise an OSError when open is called
    mock_make_dir.side_effect = OSError("Write denied")
    result = load_system_yaml(TEST_PATH, True)

    mock_make_dir.assert_called_once()
    # mock_open_func.assert_called_once_with(dummy_path, 'x', encoding='utf-8')
    assert result is None


@patch("os.path.exists")
@patch("os.makedirs")
@patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_os_error(mock_open_func, mock_make_dir, mock_exists):
    mock_exists.return_value = False
    dummy_path = '/some/path'
    # Set up the mock to raise an OSError when open is called
    mock_open_func.side_effect = OSError("Write denied")
    result = load_system_yaml(TEST_PATH, True)

    mock_make_dir.assert_called_once()
    mock_open_func.assert_called_once_with(TEST_PATH, 'x', encoding='utf-8')
    assert result is None


@patch("os.path.exists")
@patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_yaml_error(mock_open_func, mock_exists):
    mock_exists.return_value = True
    dummy_path = '/some/path'
    # Set up the mock to raise an OSError when open is called
    mock_open_func.side_effect = YAMLError
    result = load_system_yaml(TEST_PATH, False)

    mock_open_func.assert_called_once_with(TEST_PATH, 'r', encoding='utf-8')
    assert result is None

@patch("os.path.exists")
@patch('builtins.open', new_callable=mock_open)
def test_load_system_yaml_exception(mock_open_func, mock_exists):
    mock_exists.return_value = True
    dummy_path = '/some/path'
    # Set up the mock to raise an OSError when open is called
    mock_open_func.side_effect = OSError("Read denied")
    result = load_system_yaml(TEST_PATH, False)

    mock_open_func.assert_called_once_with(TEST_PATH, 'r', encoding='utf-8')
    assert result is None


@patch('os.path.expanduser')
@patch('telemetry.load_system_yaml')
def test_config_generator(mock_load, mock_expand):
    mock_load.return_value = {}
    mock_expand.side_effect = lambda x: x

    for i, res in enumerate(config_generator(False)):
        assert res[0] == {}
        assert res[1] == f"{FILE_PATH_LIST[i]}{SYSTEM_FILE_NAME}"

    assert mock_load.call_count == 4


@patch('os.path.expanduser')
@patch('telemetry.load_system_yaml')
def test_config_generator_isolation_series(mock_load, mock_expand):
    mock_load.return_value = {}
    mock_expand.side_effect = lambda x: x

    i = 0
    for res in config_generator(False):
        assert res[0] == {}
        assert res[1] == f"{FILE_PATH_LIST[i]}{SYSTEM_FILE_NAME}"
        i += 1
        if i > 1:
            break

    i = 0
    for res in config_generator(True):
        assert res[0] == {}
        assert res[1] == f"{FILE_PATH_LIST[i]}{SYSTEM_FILE_NAME}"
        i += 1

    assert mock_load.call_count == 6


@patch('os.path.expanduser')
@patch('telemetry.load_system_yaml')
def test_config_generator_isolation_parallel(mock_load, mock_expand):
    mock_load.return_value = {}
    mock_expand.side_effect = lambda x: x

    for i, res in enumerate(config_generator(False)):
        assert res[0] == {}
        assert res[1] == f"{FILE_PATH_LIST[i]}{SYSTEM_FILE_NAME}"
        for j, res in enumerate(config_generator(True)):
            assert res[0] == {}
            assert res[1] == f"{FILE_PATH_LIST[j]}{SYSTEM_FILE_NAME}"

    assert mock_load.call_count == 20


@patch('builtins.open', new_callable=mock_open, read_data=f'{DEPLOYMENT_TYPE_KEY}: docker')
@patch('yaml.safe_dump')
@patch('os.path.expanduser')
@patch('telemetry.load_system_yaml')
def test_persist_uuid_success(mock_load, mock_expand, mock_dump, mock_open_func):
    mock_load.return_value = {}
    mock_expand.side_effect = lambda x: x
    target_path = f"{FILE_PATH_LIST[0]}{SYSTEM_FILE_NAME}"
    persist_uuid(TEST_UUID)
    mock_load.assert_called_once()
    mock_dump.assert_called_once_with({f'{UUID_KEY}':TEST_UUID}, ANY, default_flow_style=False)
    mock_open_func.assert_called_once_with(target_path, 'w', encoding='utf-8')


@patch('builtins.open', new_callable=mock_open)
@patch('telemetry.load_system_yaml')
def test_persist_uuid_os_error(mock_load, mock_open_func):
    mock_load.return_value = {}
    mock_open_func.side_effect = OSError("Read denied")
    persist_uuid(TEST_UUID)
    assert mock_load.call_count == 4
    assert mock_open_func.call_count == 4


@patch('telemetry.load_uuid')
def test_init_telemetry_with_uuid(mock_load_uuid):
    mock_load_uuid.return_value = TEST_UUID
    instance = _Telemetry()

    mock_load_uuid.assert_called_once()
    assert instance.uuid == TEST_UUID


@patch('telemetry.persist_uuid')
@patch('telemetry.load_uuid')
def test_init_telemetry_no_uuid(mock_load_uuid, mock_persist):
    mock_load_uuid.return_value = None
    instance = _Telemetry()

    mock_load_uuid.assert_called_once()
    mock_persist.assert_called_once()
    assert instance.uuid is not None


@patch('telemetry.load_uuid')
def test_get_instance(mock_load_uuid):
    mock_load_uuid.return_value = TEST_UUID
    instance1 = get_telemetry_instance()
    instance2 = get_telemetry_instance()

    mock_load_uuid.assert_called_once()
    assert instance1.uuid == TEST_UUID
    assert instance2.uuid == TEST_UUID
    assert instance1 is instance2


@patch('telemetry.load_system_yaml')
def test_load_uuid_success(mock_yaml):
    mock_yaml.return_value = {UUID_KEY : TEST_UUID}

    result = load_uuid()

    assert isinstance(result, UUID)
    assert TEST_UUID == str(result)


@patch('telemetry.load_system_yaml')
def test_load_uuid_malformed(mock_yaml):
    mock_yaml.return_value = {UUID_KEY : "abcd-1234"}

    result = load_uuid()

    assert result is None
    assert mock_yaml.call_count == 4


@patch('telemetry.load_system_yaml')
def test_load_uuid_not_available(mock_yaml):
    mock_yaml.return_value = {DEPLOYMENT_TYPE_KEY : "docker"}

    result = load_uuid()

    assert result is None
    assert mock_yaml.call_count == 4


@patch('telemetry.load_uuid')
def test_collect_telemetry_no_env_variables(mock_load_uuid):
    mock_load_uuid.return_value = TEST_UUID
    instance = _Telemetry()

    result = instance.collect_telemetry()
    mock_load_uuid.assert_called_once()
    assert API_VERSION_KEY not in result
    assert DEPLOYMENT_TYPE_KEY not in result
    assert TELEMETRY_APP_KEY in result
    assert UUID_KEY in result
    assert result[UUID_KEY] == TEST_UUID
    assert result[TELEMETRY_APP_KEY] == APP_NAME


@patch('os.getenv')
@patch('telemetry.load_uuid')
def test_collect_telemetry_all_env_variables(mock_load_uuid, mock_getenv):
    mock_load_uuid.return_value = TEST_UUID
    def side_effect_func(key):
        if key == API_VERSION_KEY:
            return TEST_API_VERSION
        elif key == DEPLOYMENT_TYPE_KEY:
            return TEST_DEPLOY_TYPE
        else:
            return None
    mock_getenv.side_effect = side_effect_func
    instance = _Telemetry()

    result = instance.collect_telemetry()
    mock_load_uuid.assert_called_once()
    assert API_VERSION_KEY in result
    assert DEPLOYMENT_TYPE_KEY in result
    assert TELEMETRY_APP_KEY in result
    assert UUID_KEY in result
    assert result[API_VERSION_KEY] == TEST_API_VERSION
    assert result[DEPLOYMENT_TYPE_KEY] == TEST_DEPLOY_TYPE
    assert result[UUID_KEY] == TEST_UUID
    assert result[TELEMETRY_APP_KEY] == APP_NAME


@patch('os.getenv')
@patch('telemetry.load_uuid')
def test_collect_telemetry_deploy_type(mock_load_uuid, mock_getenv):
    mock_load_uuid.return_value = TEST_UUID
    def side_effect_func(key):
        if key == DEPLOYMENT_TYPE_KEY:
            return TEST_DEPLOY_TYPE
        else:
            return None
    mock_getenv.side_effect = side_effect_func
    instance = _Telemetry()

    result = instance.collect_telemetry()
    mock_load_uuid.assert_called_once()
    assert API_VERSION_KEY not in result
    assert DEPLOYMENT_TYPE_KEY in result
    assert TELEMETRY_APP_KEY in result
    assert UUID_KEY in result
    assert result[DEPLOYMENT_TYPE_KEY] == TEST_DEPLOY_TYPE
    assert result[UUID_KEY] == TEST_UUID
    assert result[TELEMETRY_APP_KEY] == APP_NAME


@patch('os.getenv')
@patch('telemetry.load_uuid')
def test_collect_telemetry_api_version(mock_load_uuid, mock_getenv):
    mock_load_uuid.return_value = TEST_UUID
    def side_effect_func(key):
        if key == API_VERSION_KEY:
            return TEST_API_VERSION
        else:
            return None
    mock_getenv.side_effect = side_effect_func
    instance = _Telemetry()

    result = instance.collect_telemetry()
    mock_load_uuid.assert_called_once()
    assert API_VERSION_KEY in result
    assert DEPLOYMENT_TYPE_KEY not in result
    assert TELEMETRY_APP_KEY in result
    assert UUID_KEY in result
    assert result[API_VERSION_KEY] == TEST_API_VERSION
    assert result[UUID_KEY] == TEST_UUID
    assert result[TELEMETRY_APP_KEY] == APP_NAME


@patch('builtins.print')
@patch('telemetry.load_uuid')
@patch('telemetry._Telemetry.collect_telemetry')
@patch('telemetry.create_telemetry_url')
@patch('requests.get')
def test_report_basic_telemetry_success(mock_get, mock_url, mock_collect, mock_uuid, mock_print):
    mock_uuid.return_value = TEST_UUID
    mock_collect.return_value = {}
    mock_url.return_value = TEST_URL
    test_response = Response()
    test_response.status_code = 200
    mock_get.return_value = test_response

    instance = _Telemetry()
    instance.report_basic_telemetry()

    mock_uuid.assert_called_once()
    mock_get.assert_called_once()
    mock_print.assert_called_once_with("Sent telemetry successfully")


@patch('builtins.print')
@patch('telemetry.load_uuid')
@patch('telemetry._Telemetry.collect_telemetry')
@patch('telemetry.create_telemetry_url')
@patch('requests.get')
def test_report_basic_telemetry_send_fail(mock_get, mock_url, mock_collect, mock_uuid, mock_print):
    mock_uuid.return_value = TEST_UUID
    mock_collect.return_value = {}
    mock_url.return_value = TEST_URL
    test_response = Response()
    test_response.status_code = 404
    mock_get.return_value = test_response

    instance = _Telemetry()
    instance.report_basic_telemetry()

    mock_uuid.assert_called_once()
    mock_get.assert_called_once()
    expected_msg = f"Request failed with status code {test_response.status_code}"
    mock_print.assert_called_once_with(expected_msg)


@patch('builtins.print')
@patch('telemetry.load_uuid')
@patch('telemetry._Telemetry.collect_telemetry')
@patch('telemetry.create_telemetry_url')
@patch('requests.get')
def test_report_basic_telemetry_timeout(mock_get, mock_url, mock_collect, mock_uuid, mock_print):
    mock_uuid.return_value = TEST_UUID
    mock_collect.return_value = {}
    mock_url.return_value = TEST_URL
    mock_get.side_effect = Timeout()

    instance = _Telemetry()
    instance.report_basic_telemetry()

    mock_uuid.assert_called_once()
    mock_get.assert_called_once()
    mock_print.assert_called_once_with("Telemetry request timed out")
