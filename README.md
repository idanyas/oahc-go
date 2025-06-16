# OCI ARM Host Capacity Finder (Go Version)

This project is a Go rewrite of the popular `hitrov/oci-arm-host-capacity` PHP script. It helps you acquire an "Always Free" Oracle Cloud Infrastructure (OCI) Ampere A1 compute instance by repeatedly trying to create one until capacity is available.

The script is designed to be run as a cron job or a scheduled task. When it detects that OCI has freed up capacity in your home region, it will automatically create an instance with your specified configuration and can notify you via Telegram.

This version is a single, self-contained binary with no external dependencies, making it fast, secure, and easy to deploy.

## Features

- Periodically checks for available OCI capacity for Ampere A1 (or other shapes).
- Automatically creates an instance upon success.
- Supports custom shapes, OCPU count, and memory.
- Can create instances from a backup boot volume.
- Optional Telegram notifications on success.
- Handles OCI rate limiting (`TooManyRequests`) by backing off automatically.
- Zero third-party library dependencies for core functionality.
- Configuration via a simple `.env` file and/or environment variables.

## Prerequisites

1.  A working Go environment (1.18+ recommended).
2.  An Oracle Cloud Infrastructure account.
3.  An OCI API key pair. Follow the official [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm) to generate one.

## Installation

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/idanyas/oahc-go.git
    cd oahc-go
    ```

2.  **Build the binary:**
    ```bash
    go build
    ```
    This will create an executable file named `oahc-go` (or `oahc-go.exe` on Windows) in the current directory.

## Configuration

Configuration is managed through a `.env` file.

1.  **Copy the example file:**

    ```bash
    cp .env.example .env
    ```

2.  **Edit the `.env` file** with your specific OCI details. All parameters are required unless marked as optional.

    - `OCI_REGION`: Your OCI home region (e.g., `us-ashburn-1`).
    - `OCI_USER_ID`: Your OCI User OCID.
    - `OCI_TENANCY_ID`: Your Tenancy OCID.
    - `OCI_KEY_FINGERPRINT`: The fingerprint of your uploaded API public key.
    - `OCI_PRIVATE_KEY_FILENAME`: The absolute path to your API private key (`.pem`) file.
    - `OCI_SUBNET_ID`: The OCID of the subnet where the instance will be created.
    - `OCI_IMAGE_ID`: The OCID of the image to use for the instance. (Required if not using `OCI_BOOT_VOLUME_ID`).
    - `OCI_SSH_PUBLIC_KEY`: The full content of your public SSH key (e.g., from `~/.ssh/id_rsa.pub`). **Must be on a single line.**
    - `OCI_SHAPE`: (Optional, defaults to `VM.Standard.A1.Flex`) The shape of the instance.
    - `OCI_OCPUS`: (Optional, defaults to `4`) The number of OCPUs.
    - `OCI_MEMORY_IN_GBS`: (Optional, defaults to `24`) The amount of memory in GB.
    - `OCI_AVAILABILITY_DOMAIN`: (Optional) A specific availability domain to try. If empty, the script will try all ADs in your region. You can also provide a JSON array of strings: `["AD_1_OCID", "AD_2_OCID"]`.
    - `OCI_BOOT_VOLUME_ID`: (Optional) The OCID of an existing boot volume to create the instance from.
    - `OCI_BOOT_VOLUME_SIZE_IN_GBS`: (Optional) The size of the boot volume in GBs (minimum 50).
    - `OCI_MAX_INSTANCES`: (Optional, defaults to `1`) The maximum number of instances of this shape to allow before stopping.
    - `TELEGRAM_BOT_API_KEY`: (Optional) Your Telegram Bot API key for notifications.
    - `TELEGRAM_USER_ID`: (Optional) Your Telegram User/Chat ID for notifications.
    - `OCI_JSON_LOG_PATH`: (Optional) Path to a file for logging API responses in JSON format. It logs all instance creation attempts (success or failure) and any other failed API calls. Example: `/var/log/oahc-go/api.log`. Note: Ensure the running user has write permissions to the specified path.

## Running the Script

Execute the binary from your terminal:

```bash
./oahc-go
```

If your `.env` file is located elsewhere, you can specify its path with the `-envfile` flag:

```bash
./oahc-go -envfile /path/to/my.env
```

The script will log its progress to standard output. If it finds capacity, it will create the instance, print the details, and exit. If not, it will print the "Out of host capacity" message for each availability domain it tries.

## Periodic Job Setup (Cron)

To run the script automatically, set up a cron job.

1.  Open your crontab for editing:

    ```bash
    crontab -e
    ```

2.  Add a new line to run the script every minute. Make sure to use absolute paths for the binary and its log file.

    ```cron
    * * * * * /path/to/your/oahc-go/oahc-go >> /path/to/your/oahc-go/oahc.log 2>&1
    ```

    This will run the script every minute and append its output to `oahc.log`.

## How It Works

1.  **Load Config:** Reads your settings from the `.env` file.
2.  **Check Existing Instances:** Queries OCI to see if you already have an instance of the target shape running. If you've reached `OCI_MAX_INSTANCES`, it exits.
3.  **Find Availability Domain (AD):** Determines which ADs to check.
4.  **Loop and Create:** Iterates through the ADs and sends a `CreateInstance` API request for each one.
    - **On Success:** The instance is created. The script prints the details, sends a Telegram notification (if configured), and exits.
    - **On "Out of host capacity":** The script logs the message, waits a few seconds, and moves to the next AD.
    - **On "Too Many Requests":** The script logs a message and creates a temporary file to prevent it from running again for a configurable amount of time (default 5 minutes). This respects OCI's rate limits.
    - **On Other Errors:** The script exits with a fatal error, as this likely indicates a configuration problem that needs to be fixed.