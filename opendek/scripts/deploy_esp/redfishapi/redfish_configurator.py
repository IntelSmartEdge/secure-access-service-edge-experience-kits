# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation

"Provides class for specific system configuration via redfish api"

import time
import logging
import threading
import redfishapi

logger = logging.getLogger(__name__)
if not logger.handlers:
    handler = logging.StreamHandler()
    handler.setFormatter(logging.Formatter("|%(asctime)s.%(msecs)03d|"
        "%(levelname)s:%(name)s:%(threadName)s: %(message)s"))
    logger.setLevel(min(logging.INFO, logging.root.level))
    logger.addHandler(handler)

class BmcHost:
    """Data class representing bmc host.

       Args:
            address: Bmc address to be passed to RedfishAPI class see implementation for details.
            username: Bmc username to be passed to RedfishAPI class
            password: Bmc password to be passed to RedfishAPI class
            features: A dictionary contating keys:
                        'tpm'(Trusted Platform Enable),
                        'sb' (Secure Boot),
                        'sgx' (Software Guard Extensions),
                        'autoMount' # TODO: Describe and possibly change name.
                      The values should be one of three possiblities:
                        None - Do not modify feature value (default)
                        'on' (str) - Enable feature during configuration
                        'off' (str) - Disable feature during configuration

                      # TODO: Document how args should look for 'autoMount' (or add other parameters for it)

    """
    def __init__(self,
                 address=None,
                 username=None,
                 password=None,
                 features=None):

        self.address = address
        self.username = username
        self.password = password
        self.features = {"tpm": None,
                         "sb": None,
                         "sgx": None,
                         "autoMount": None}
        if features:
            self.features.update(features)


class RedfishConfigurator:
    """Configuring hosts into desired state via redfish.

    Attributes:
        hosts: iterable of hosts (BmcHost class) to be configured into desired state.
    """
    SGX_KEYS = ["MemOpMode", "NodeInterleave", "MemoryEncryption", "IntelSgx", "SgxFactoryReset", "PrmrrSize"]

    def __init__(self, hosts, debug=False):
        self.hosts = hosts
        if debug:
            logger.setLevel(logging.DEBUG)
            redfishapi.enable_debug() # pylint: disable=E1101
        self._error_lock = threading.Lock()
        self._failure_info = {}

    def _set_failure(self, host: BmcHost, exception: Exception):
        """Thread safe failure reason setter.

           Allows threads to save exception that caused fail for later analysis.
        """
        with self._error_lock:
            self._failure_info[host.address] = exception

    @property
    def failed_hosts(self) -> dict:
        "Returns dictionary contating host address as key and exception as value."
        with self._error_lock:
            return self._failure_info

    def configure_bios_settings(self, parallel=True):
        """Runs configuration of bios settings for each host.

           This will configure features such as enablig / disabling tpm, sgx, sb.

           Args:
                parallel: Flag pointing if hosts should be configured parallel using threads.
                          If False hosts will be configured one by one.
        """
        def single_host_configure(host):
            logger.info("Starting bios configuration for host with address: %s.", host.address)
            try:
                # pylint: disable=E1101
                rapi = redfishapi.RedfishAPI(host.address, host.username, host.password)
                try:
                    rapi.check_connectivity(error_passthrough=True)
                except Exception as e:
                    raise Exception("Redfish api does not work, ensure address and credentials right.") from e

                # A list of dictionaries containing properties to be applied during single stage
                all_stages = []

                def add_stages(new_stages):
                    "Extends all_stages or joins new with already created"
                    if len(all_stages) < len(new_stages):
                        all_stages.extend({} for i in range(len(new_stages) - len(all_stages)))
                    for current, new in zip(all_stages, new_stages):
                        current.update(new)

                for feature, state in host.features.items():
                    if state is None:
                        continue

                    state = state.lower()

                    if feature == "sgx":
                        if state == "on":
                            add_stages([{"MemOpMode": "OptimizerMode", "NodeInterleave": "Disabled"},
                                        {"MemoryEncryption": "SingleKey"},
                                        {"IntelSgx": "On", "SgxFactoryReset": "On"},
                                        {"PrmrrSize": "64G"}])
                        elif state == "off":
                            add_stages([{"IntelSgx": "Off"}])
                    elif feature == "tpm":
                        if state == "on":
                            add_stages([{"TpmSecurity": "On"}])
                        elif state == "off":
                            add_stages([{"TpmSecurity": "Off"}])
                    elif feature == "sb":
                        if state == "on":
                            add_stages([{"SecureBoot": "Enabled"}])
                        elif state == "off":
                            add_stages([{"SecureBoot": "Disabled"}])

                all_stages = self.validate_pending_stages(rapi, all_stages)

                for stage in all_stages:

                    if not stage:
                        continue

                    logger.info("Applying stage values: %s.", stage)
                    rapi.set_bios_attributes(stage)
                    logger.info("Creating config job.")
                    rapi.finalize_bios_settings()

                    if rapi.reboot_required:
                        logger.info("Rebooting server.")
                        self.reboot_server(rapi)

                        job_id = rapi.get_pending_config_jobs()[0]["Id"]
                        logger.info("Waiting for job: %s to finish.", job_id)
                        self.wait_for_job_finished(rapi, job_id=job_id)

                    logger.info("Checking if stage correctly applied.")

                    self.check_stage_applied(rapi, stage)
                    logger.info("Stage successfully applied.")

                    # This fixed sleep should be percieved as workaround.
                    # If we perform stages too quickly then
                    # PATCH /redfish/v1/Systems/System.Embedded.1/Bios/Settings fails with 503 code.
                    # Message: "Unable to apply the configuration changes because an
                    # import or export operation is currently in progress."
                    # and "Resolution": "Wait for the current import or export
                    # operation to complete and retry the operation.
                    # If the issue persists, contact your service provider."
                    # To be found what are those import/export ops and how can we check their state.
                    time.sleep(15)

            except Exception as e:
                self._set_failure(host, e)
                raise e
            logger.info("Host successfully configured.")

        if not parallel:
            for host in self.hosts:
                single_host_configure(host)
            return

        threads = []
        for host in self.hosts:
            th = threading.Thread(target=single_host_configure, args=(host,), name=host.address)
            th.start()
            threads.append(th)

        for thread in threads:
            thread.join()

    @staticmethod
    def wait_for_power_state(host_rapi, power_state, timeout=60, check_every=2):
        """Waits for system power state change.

        Atrributes:
            power_state: 'On' or 'Off'
            timeout: Time to wait until system is in desired state in seconds.
                     There is one check per second.
        """
        for _ in range(timeout):
            if host_rapi.system_info["PowerState"] == power_state:
                break
            time.sleep(check_every)
        else:
            return False
        return True

    def reboot_server(self, host_rapi):
        "Reboots given system."

        if host_rapi.system_info["PowerState"] == "On":
            logger.info("GracefulShutdown server")
            host_rapi.system_reset_action("GracefulShutdown")

            logger.info("Waiting for server to go Off")
            if not self.wait_for_power_state(host_rapi, "Off"):
                logger.warning("Server did not gracefully close within time limit, forcing...")
                host_rapi.system_reset_action("ForceOff")

            if not self.wait_for_power_state(host_rapi, "Off"):
                raise Exception("Wait timeout exceeded for server to be Off.")

        logger.info("Power On server")
        host_rapi.system_reset_action("On")

        logger.info("Waiting for server to go On")
        if not self.wait_for_power_state(host_rapi, "On"):
            raise Exception("Wait timeout exceeded for server to be On.")

    @staticmethod
    def wait_for_job_finished(host_rapi, job_id, timeout=1800, check_every=12):
        "Waits for job with specific id to be finished."

        prev_percentage = None
        for _ in range(timeout):
            job_data = host_rapi.get_job_info(job_id)

            if "PercentComplete" in job_data and job_data["PercentComplete"] != prev_percentage:
                prev_percentage = job_data["PercentComplete"]
                logger.info("Job: %s complete percent: %s", job_id, prev_percentage)

            time.sleep(check_every)

            job_state = job_data["JobState"]
            if job_state in ["Scheduled", "Running",
                            "New", "Scheduling",
                            "ReadyForExecution", "Waiting"]:
                continue

            if job_state in ["Failed", "CompletedWithErrors", "RebootFailed"]:
                msg = ["Job does not succedded.",
                       "Job details:"] + [f"{k}: {v}" for k, v in job_data.items()]
                raise Exception("\n\t".join(msg))

            if job_state == "Completed":
                break
        else:
            msg = ["Job does not succedded within given interval.",
                   f"JobState {job_state}",
                   "Job details:"] + [f"{k}: {v}" for k, v in job_data.items()]
            raise Exception("\n\t".join(msg))

    @staticmethod
    def validate_pending_stages(host_rapi, stages: list) -> list:
        """Validates changes that user want to apply to given host.

           Validation consists of getting currently set paramerters
           and comparing them with parameters to be applied in each stage.

           Args:
                host_rapi: Host to validate bios parameters from.
                stages: list of dictionaries containing parameters to be applied during
                        each stage.

           Returns:
                list: Filtered stages - contating only those which are not yet applied.
        """
        current = host_rapi.bios_attributes
        valid_stages = []
        found_sgx_keys = []

        for id_, stage in enumerate(stages):
            not_applied = {}
            for key, val in stage.items():
                if not key in current or current[key] != val:
                    not_applied[key] = val
                    if key in RedfishConfigurator.SGX_KEYS:
                        found_sgx_keys.append((key, id_))
            valid_stages.append(not_applied)

        # SgxFactoryReset is a special sgx feature attribute: it should be set to 'On' when applying,
        # but when reading it will always be set to 'Off', therefore ignore its value here.
        if len(found_sgx_keys) == 1 and found_sgx_keys[0][0] == "SgxFactoryReset":
            id_ = found_sgx_keys[0][1]
            del valid_stages[id_]["SgxFactoryReset"]
            if not valid_stages[id_]:
                del valid_stages[id_]

        return valid_stages

    @staticmethod
    def check_stage_applied(host_rapi, stage_data, timeout=20):
        "Check if bios attributes endpoint available and has desired values set."

        # After reboot and config job apply idrac becomes unresponsive for some time
        # resulting in 500 Server error and message
        # iDRAC is currently unable to display any information because data sources are unavailable.
        endpoint = f"/Systems/{host_rapi.system_id}/Bios"
        logger.info("Waiting for endpoint to be availble: %s ...", endpoint)
        check_every = 2
        for _ in range(timeout):
            if host_rapi.check_connectivity(endpoint=endpoint):
                break
            time.sleep(check_every)
        else:
            host_rapi.check_connectivity(endpoint=endpoint,
                                        error_passthrough=True)

        logger.info("Endpoint, available checking values applied...")

        # Gets attributes from server and updates cached values
        current = host_rapi.get_bios_attributes()
        not_applied = {sk: sv for sk, sv in stage_data.items()
                       if sk not in current or
                       current[sk] != sv}

        # SgxFactoryReset property is always "off"
        # it should be applied one time "on" to reset sgx settings / keys
        if "SgxFactoryReset" in not_applied:
            del not_applied["SgxFactoryReset"]

        if not_applied:
            msg = ["Failed to apply following bios settings:"] + \
                  [f"{k}: {v}" for k, v in not_applied.items()]
            raise Exception("\n\n".join(msg))
