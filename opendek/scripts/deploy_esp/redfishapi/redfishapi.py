#!/usr/bin/env python3

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation

"Provides selected configuration possibilites via REDFISH REST management api."

import json
import sys
import warnings
import argparse
import logging
import requests # pylint: disable=import-error


warnings.filterwarnings("ignore")

logger = logging.getLogger(__name__)
if not logger.handlers:
    handler = logging.StreamHandler()
    handler.setFormatter(logging.Formatter("|%(asctime)s.%(msecs)03d|%(levelname)s:%(name)s: %(message)s"))
    logger.setLevel(min(logging.INFO, logging.root.level))
    logger.addHandler(handler)

class NoTpmModuleException(KeyError):
    "To be raised in case user performs tpm operation on system that does not have tpm module."

def enable_debug():
    "Set loggers level to debug"
    logger.setLevel(logging.DEBUG)
    log = logging.getLogger('urllib3')
    log.setLevel(logging.DEBUG)
    stream = logging.StreamHandler()
    stream.setFormatter(logging.Formatter("%(levelname)s:%(name)s: %(message)s"))
    stream.setLevel(logging.DEBUG)
    log.addHandler(stream)

def join_url(*pieces):
    "Single url join splited url."
    return "/".join(s.strip("/") for s in pieces)


class RedfishAPI:
    "Redfish api rest wrapper."

    # pylint: disable=too-many-instance-attributes, too-many-public-methods
    # The current number is reasonable

    # How many seconds to wait for the server to send data before giving up
    HTTP_RESPONSE_TIMEOUT = 10

    def __init__(self, base_url, username, password, proxy=None, verbose=False):
        self.base_url = "https://" + join_url(base_url, "/redfish/v1")
        self.username = username
        self.password = password
        self.proxy = proxy
        self.verbose = verbose
        self._system_id = None
        # Attributes to be applied
        self._pending_bios_attrs = {}
        # Currently set attributes
        self._bios_attributes = {}
        self._reboot_required = False
        self._session = requests.Session()
        self._session.auth = (username, password)

    def _request(self, method: str, endpoint: str, check=True, timeout=HTTP_RESPONSE_TIMEOUT, **kwargs):
        """Generic http request.

        Note:
            First argument method is case sensitive it has to be exact method
            name as in requests library.
        Args:
            method: String name of http method available in
                requests library. Possible values: get, post, put, patch, delete.
            endpoint: API Path to be appended to url/redfish/v1
            check: Whether check response for accepted http status
                code (2xx/3xx) and exit 1 if bad status code occurs.
            kwargs: Extra named parameters to pass for requests
                http call.
        Returns:
            requests.Response: The Response object, which contains a server's response to an HTTP request.
        """
        def print_extended_info(response):
            try:
                logger.error("Extended Info Message: %s", json.dumps(response.json(), indent=2))
            except Exception as e:
                logger.error("Can't decode extended info message. Exception: %s", e)

        def check_response(response):
            try:
                response.raise_for_status()
            except requests.exceptions.HTTPError as e:
                logger.error("Request returned inccorect response code: %s", e)
                print_extended_info(response)
                raise e

        if "data" in kwargs:
            kwargs["data"] = json.dumps(kwargs["data"])

        # gets http method attribute by name and calls it
        response = getattr(self._session, method)(join_url(self.base_url, endpoint),
                                             verify=False,  # nosec
                                             auth=(self.username, self.password),
                                             proxies=self.proxy,
                                             timeout=timeout,
                                             **kwargs)
        if check:
            check_response(response)

        if self.verbose:
            print_extended_info(response)

        return response


    @property
    def system_id(self) -> str:
        """Returns id of first system managed by interface.

        Note:
            Assumes there will be only single member.
            Redfish api host system id can vary depending on platform.
            Supermicro system_id is usually '1' and dell 'System.Embedded.1'.
        """
        if not self._system_id:
            response = self._request("get", "/Systems/")
            self._system_id = str(response.json()["Members"][0]["@odata.id"]).replace("/redfish/v1/Systems/", "")
        return self._system_id

    @property
    def is_supermicro(self) -> bool:
        "To check if system_id is supermicro/redfish standard specific"
        return self.system_id == "1"

    @property
    def is_dell(self) -> bool:
        "To check if system_id is dell specific"
        return self.system_id == "System.Embedded.1"

    @property
    def reboot_required(self) -> bool:
        "Getter for reboot required attribute"
        return self._reboot_required

    @property
    def bios_attributes(self) -> dict:
        """Returns dictionary with system bios attributes.

           To lower redundant GET requests attributes are cached
           to update cached value `self.get_bios_attributes()` should be called.
        """
        if not self._bios_attributes:
            self.get_bios_attributes()
        return self._bios_attributes

    @property
    def system_info(self) -> dict:
        "Returns dictionary with ComputerSystem schema attributes"
        response = self._request("get", f"/Systems/{self.system_id}")
        return response.json()

    def check_connectivity(self, endpoint="", error_passthrough=False):
        """Checks if base url https://address/redfish/v1 is accesible
        for further redfish operations.
        """
        try:
            self._request("get", endpoint, timeout=10)
        except requests.exceptions.RequestException as e:
            if error_passthrough:
                raise e
            return False
        return True

    def patch_secure_boot(self, payload_data):
        """Sends PATCH rest request to system /SecureBoot endpoint.

        Note:
            This call will automatically create config job that will update
            set values opposite to calls to /Bios/Settings (for dell).
        Args:
            payload_data dict: Dictionary data to be send as json.

        Returns:
            requests.Response object containing response to patch rest call
        """
        headers = {"content-type": "application/json"}
        return self._request("patch",
                             f"/Systems/{self.system_id}/SecureBoot",
                             headers=headers,
                             data=payload_data)

    def patch_bios_settings(self, payload_data):
        """Sends PATCH rest request to system /Bios endpoint.

        Args:
            payload_data dict: Dictionary data to be send as json.

        Returns:
            requests.Response object containing response to patch rest call
        """
        headers = {"content-type": "application/json"}
        return self._request("patch",
                             f"/Systems/{self.system_id}/Bios/Settings",
                             headers=headers,
                             data=payload_data)

    def enable_secure_boot(self):
        """Enables secure boot for given system.

        Note:
            For changes to be to applied for dell finalize_bios_settings should be called.
        """
        # supermicro has different payload
        if self.is_supermicro:
            payload = {"SecureBoot": "Enabled"}
            response = self.patch_secure_boot(payload)
            logger.info("Secure boot command successful, code return is %s", response.status_code)
        else:
            self._pending_bios_attrs["SecureBoot"] = "Enabled"

    def disable_secure_boot(self):
        """Disables secure boot for given system.

        Note:
            For changes to be to applied for dell finalize_bios_settings should be called.
        """
        # supermicro has different payload
        if self.is_supermicro:
            payload = {"SecureBoot": "Disabled"}
            response = self.patch_secure_boot(payload)
            logger.info("Secure boot command successful, code return is %s", response.status_code)
        else:
            self._pending_bios_attrs["SecureBoot"] = "Disabled"

    def get_secure_boot_enable_status(self) -> bool:
        """Returns bool value representing state of secure boot.

        Returns:
            True if secure boot is enabled for system otherwise
            False.
        """
        response = self._request("get", f"/Systems/{self.system_id}/SecureBoot")
        data = response.json()
        # supermicro option "SecureBoot" is string while dell store bools in json
        return data["SecureBoot"] == "Enabled" if self.is_supermicro else data["SecureBootEnable"]

    def create_bios_config_job(self) -> requests.Request:
        """Creates bios config job for dell iDRAC.
        For applying changes added to /Bios/Settings endpoint.
        """
        payload = {"TargetSettingsURI": "/redfish/v1/Systems/System.Embedded.1/Bios/Settings"}
        headers = {"content-type": "application/json"}
        return self._request("post",
                             "/Managers/iDRAC.Embedded.1/Jobs",
                             headers=headers,
                             data=payload)

    def get_pending_config_jobs(self, job_type="BIOSConfiguration") -> list:
        """Returns list of jobs description dictionaries which are marked as 'Scheduled' of given type."""
        response = self._request("get", "/Managers/iDRAC.Embedded.1/Jobs?$expand=*($levels=1)")
        return [job for job in response.json()["Members"]
                if job["JobState"] == "Scheduled" and \
                    job["JobType"] == job_type]

    def delete_pending_config_jobs(self):
        "Deletes pending config jobs."
        # should be single job, but will loop for
        for pending in self.get_pending_config_jobs():
            logger.info("Deleting job: %s", pending['Id'])
            self._request("delete", f"/Managers/iDRAC.Embedded.1/Jobs/{pending['Id']}")

    def get_job_info(self, job_id) -> dict:
        "Returns job dictionary description"
        response = self._request("get", f"/Managers/iDRAC.Embedded.1/Jobs/{job_id}")
        return response.json()

    def enable_tpm(self):
        """Enables trusted platform module support for given system.
        Note: For changes to be applied finalize_bios_settings should be called.
        """
        # TODO: Check what is payload/endpoint for supermicro
        if self.is_supermicro:
            raise NotImplementedError("Not yet implemented for supermicro.")
        self._pending_bios_attrs["TpmSecurity"] = "On"

    def disable_tpm(self):
        """Disables trusted platform module support for given system.
        Note: For changes to be applied finalize_bios_settings should be called.
        """
        # TODO: Check what is payload/endpoint for supermicro
        if self.is_supermicro:
            raise NotImplementedError("Not yet implemented for supermicro.")

        self._pending_bios_attrs["TpmSecurity"] = "Off"

    def get_tpm(self) -> bool:
        """Returns bool value representing status of system trusted platform module support.

        Returns:
            True if trusted platform module support is enabled for system otherwise
            False.
        """
        # TODO: Check what is payload/endpoint for supermicro
        if self.is_supermicro:
            raise NotImplementedError("Not yet implemented for supermicro.")

        if "TpmSecurity" not in self.bios_attributes:
            raise NoTpmModuleException("No 'TpmSecurity' found in system bios attributes. "
                                       "Please ensure tpm module installed on the system.")

        return self.bios_attributes["TpmSecurity"] == "On"

    def get_sgx(self) -> bool:
        "Returns bool value representing status of system intel SGX state"
        if self.is_supermicro:
            raise NotImplementedError("Not yet implemented for supermicro.")

        return self.bios_attributes["IntelSgx"] == "On"

    def set_bios_attributes(self, bios_attributes: dict):
        """Updates pending attributes dictionary.

        Note:
            For changes to be to applied finalize_bios_settings should be called.
        """
        self._pending_bios_attrs.update(bios_attributes)

    def get_bios_attributes(self):
        "Returns system currently set bios attributes and updates cached value."
        response = self._request("get", f"/Systems/{self.system_id}/Bios")
        self._bios_attributes = response.json()["Attributes"]
        return self._bios_attributes

    def system_reset_action(self, reset_type):
        """Resets system
        Attributes:
            reset_type: One of [On, ForceOff, ForceRestart, GracefulShutdown, PushPowerButton, Nmi]
        """
        endpoint = f"/Systems/{self.system_id}/Actions/ComputerSystem.Reset"
        payload = {"ResetType": reset_type}
        headers = {"content-type": "application/json"}
        return self._request("post",
                             endpoint,
                             data=payload,
                             headers=headers)

    def finalize_bios_settings(self):
        """Finalizes setting pending bios configuration by patching /Bios/Settings
        endpoint and creating bios_config_job.
        """
        if self.is_supermicro:
            raise NotImplementedError("Not yet implemented for supermicro.")

        # Check if configuration is not already satisfied
        current = self.bios_attributes
        for key, val in self._pending_bios_attrs.copy().items():
            if key in current and current[key] == val:
                del self._pending_bios_attrs[key]

        if not self._pending_bios_attrs:
            return

        # Delete pending config jobs as there can be only single one
        self.delete_pending_config_jobs()

        logger.debug("Patch with attributes: %s", self._pending_bios_attrs)
        response = self.patch_bios_settings({"Attributes": self._pending_bios_attrs})
        logger.info("Bios patch command successful, code return is %s", response.status_code)

        # Changes in dell bios settings require creation of config job for them to take effect
        self.create_bios_config_job()
        self._pending_bios_attrs = {}
        self._reboot_required = True


def parse_args():
    """Parse argument passed in stdin"""
    class CustomFormatter(argparse.ArgumentDefaultsHelpFormatter, argparse.RawDescriptionHelpFormatter):
        "Default parsers except help"
    parser = argparse.ArgumentParser(description="This script utilizes Redfish API to "
                                                 "perform management operations on "
                                                 "iDRAC or SUPERMICRO machine.",
                                                 formatter_class=CustomFormatter,
                                                 epilog="Note: Tool will not perform reboot by default. It is user "
                                                        "decision if operation performed requires it.\n"
                                                        "Get value operation ex. '--tpm get' allows only single"
                                                        "option to be taken via single call "
                                                        "(--tpm get --sb get will not work).\n\n"
                                                        "Examples:\n"
                                                        "> %(prog)s.py --sb on -u "
                                                        "calvin -p rootpass --ip 10.22.22.139 \n\n"
                                                        "> %(prog)s.py --tpm get -u "
                                                        "calvin -p rootpass --ip 10.22.22.139\n\n"
                                                        "> %(prog)s.py --tpm off --sb off -u "
                                                        "calvin -p rootpass --ip 10.22.22.139")
    parser.add_argument("--ip", help="MGMT IP address.", required=True)
    parser.add_argument("-u", "--user", help="MGMT username.", required=True)
    parser.add_argument("-p", "--password", help="MGMT password.", required=True)
    parser.add_argument("--proxy", help="Proxy server for traffic redirection.", required=False)
    parser.add_argument("-v", "--verbose",
                        help="Extend verbosity.",
                        required=False,
                        action="store_true",
                        default=False)
    parser.add_argument("--sb",
                        help="Secure boot configuration.",
                        choices=["on", "off", "get"],
                        required=False)
    parser.add_argument("--tpm",
                        help="Trusted module platform configuration.",
                        choices=["on", "off", "get"],
                        required=False)

    return parser.parse_args()


def main():
    "Main execution function"
    args = parse_args()
    if not args.tpm and not args.sb:
        logger.error("Incorrect parameters run: -h/--help")
        sys.exit(1)

    rapi = RedfishAPI(args.ip,
                      args.user,
                      args.password,
                      verbose=args.verbose)

    # try to access endpoint without proxy, if can't reach endpoint try to set proxy
    if not rapi.check_connectivity():
        if not args.proxy:
            logger.error("Redfish is inaccessible. Please ensure ip address is correct.")
            sys.exit(1)
        logger.info("Base url %s inaccessible without proxy...", rapi.base_url)
        proxy = {}
        proxy["http"] = args.proxy
        proxy["https"] = args.proxy
        rapi.proxy = proxy
        if not rapi.check_connectivity():
            logger.info("Redfish is inaccessible via proxy. Please ensure address is correct.")
            sys.exit(1)

    logger.info("Retrieved system id: %s", rapi.system_id)

    calls = {"tpm": {"on": rapi.enable_tpm,
                     "off": rapi.disable_tpm,
                     "get": rapi.get_tpm},
             "sb": {"on": rapi.enable_secure_boot,
                    "off": rapi.disable_secure_boot,
                    "get": rapi.get_secure_boot_enable_status}
             }

    #  human readable states
    hr_status = {True: "enabled", False: "disabled"}
    hr_command = {"tpm": "trusted platform module", "sb": "secure boot"}
    for command, methods in calls.items():
        option = getattr(args, command)
        if not option:
            continue

        current_status = methods["get"]()
        logger.info("Retrieved %s status: %s", hr_command[command], hr_status[current_status])

        if option == "get":
            return sys.exit(0) if current_status else sys.exit(1)
        elif option == "on":
            if current_status:
                logger.info("System has %s already enabled exiting...", hr_command[command])
                sys.exit(0)
            methods["on"]()
        elif option == "off":
            if not current_status:
                logger.info("System has %s already disabled exiting...", hr_command[command])
                sys.exit(0)
            methods["off"]()

    rapi.finalize_bios_settings()
    sys.exit(0)


if __name__ == "__main__":
    main()
