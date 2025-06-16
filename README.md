# 🤖 OCI ARM Host Capacity

A lightweight, modern, and efficient Go application that tirelessly scans Oracle Cloud Infrastructure (OCI) for available **"Always Free" Ampere A1** compute capacity.

When capacity is found, it automatically provisions an instance based on your configuration and sends you a Telegram notification. Designed to be run as a persistent service using Docker, it's the simplest way to secure your OCI ARM instance.

---

### ✨ Features

-   🤖 **Automated Provisioning**: Runs 24/7 and automatically creates an instance the moment capacity is available.
-   ⚙️ **Flexible Configuration**: Define your desired instance shape, OCPU count, memory, and boot volume.
-   🔔 **Telegram Notifications**: Get an instant alert on success with all the details of your new instance.
-   🧠 **Intelligent Backoff**: Automatically handles OCI API rate limits by waiting and retrying.
-   🐳 **Docker First**: Optimized for a simple and reliable deployment with Docker Compose.
-   🔒 **Efficient & Secure**: Compiled into a single, small binary for minimal overhead and attack surface.

### 🤔 How It Works

1.  **Load Config**: Reads your OCI and instance settings from the `.env` file.
2.  **Check Existing**: Checks if you've already reached your maximum desired instances. If so, it exits.
3.  **Scan & Create**: It loops through the availability domains in your region, attempting to create an instance.
    -   *On "Out of Capacity"*: It logs the message and immediately tries the next domain.
    -   *On "Too Many Requests"*: It waits for a dynamically increasing period before trying again.
    -   *On Success*: It creates the instance, sends a Telegram notification, and exits gracefully.

---

## 🚀 Setup and Configuration

This guide will walk you through setting up your OCI credentials and configuring the project to run with Docker.

### ✅ Prerequisites

*   An [Oracle Cloud Infrastructure](https://cloud.oracle.com/) account.
*   [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/install/) installed.
*   [OCI CLI](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm) & [`jq`](https://stedolan.github.io/jq/download/) installed.

### Step 1: Get Primary Credentials from OCI

1.  **Navigate to API Keys**:
    -   In the OCI Console, go to **Profile ➡️ My Profile ➡️ API Keys**.

2.  **Add a New API Key**:
    -   Click **Add API Key** and select **Generate API Key Pair**.
    -   Click **Download Private Key** and save the file as `oci_api_key.pem` in this project's folder.

3.  **Copy from Configuration Preview**:
    -   **Do not close the window!** Look for the **Configuration File Preview** text box.
    -   From this preview, copy the following values and save them in a temporary text file:
        -   `user` (your User OCID)
        -   `fingerprint`
        -   `tenancy` (your Tenancy OCID)
        -   `region` (e.g., `eu-frankfurt-1`)

### Step 2: Configure the OCI CLI

To find the remaining IDs, your OCI CLI needs to be authenticated. We'll do this by creating a configuration file manually.

1.  **Create the OCI directory and config file**:
    ```bash
    mkdir -p ~/.oci
    touch ~/.oci/config
    ```

2.  **Edit the config file** (`~/.oci/config`) and paste the following template into it.

    ```ini
    [DEFAULT]
    user=
    fingerprint=
    tenancy=
    region=
    key_file=
    ```

3.  **Fill in the template** with the values you copied in Step 1.
    -   For `key_file`, you must provide the **full, absolute path** to the `oci_api_key.pem` file you downloaded.
    -   *Example `key_file`: `/home/youruser/projects/oahc-go/oci_api_key.pem`*

### Step 3: Find Resource IDs with the CLI

Now that your CLI is authenticated, you can easily find the remaining IDs.

1.  **Set your Tenancy OCID** in your terminal for convenience:
    ```bash
    # Paste your Tenancy OCID here
    export TENANCY_ID="ocid1.tenancy.oc1..xxxxxx"
    ```

2.  **Run the following commands** to find an Image and Subnet ID. Pick one of each from the output.

    ```bash
    # Find an IMAGE_ID (look for Ubuntu or Oracle Linux aarch64)
    oci compute image list --all -c "$TENANCY_ID" | jq -r '.data[] | select(.["operating-system"] != "Windows") | select(.["display-name"] | contains("aarch64")) | "\(.["display-name"]): \(.id)"'

    # Find a SUBNET_ID
    oci network subnet list -c "$TENANCY_ID" | jq -r '.data[] | "\(.["display-name"]): \(.id)"'
    ```

### Step 4: Configure the Project `.env` File

You now have all the information needed. Let's fill out the project's environment file.

1.  **Create the `.env` file**:
    ```bash
    cp .env.example .env
    ```
2.  **Open and edit `.env`**, filling in all the required values you've gathered from the previous steps.

| Variable | Description | Required |
| :--- | :--- | :---: |
| `OCI_USER_ID` | The `user` value from Step 1. | ✅ |
| `OCI_TENANCY_ID` | The `tenancy` value from Step 1. | ✅ |
| `OCI_KEY_FINGERPRINT`| The `fingerprint` value from Step 1. | ✅ |
| `OCI_REGION` | The `region` value from Step 1. | ✅ |
| `OCI_PRIVATE_KEY_FILENAME`| **Must be `/app/oci_api_key.pem`**. | ✅ |
| `OCI_SUBNET_ID` | An OCID from Step 3. | ✅ |
| `OCI_IMAGE_ID` | An OCID from Step 3. | ✅ |
| `OCI_SSH_PUBLIC_KEY`| The **full content** of your public SSH key (`~/.ssh/id_rsa.pub`). | ✅ |
| `OCI_AVAILABILITY_DOMAIN` | Specific AD to try. *Leave empty to try all*. | |
| `TELEGRAM_BOT_API_KEY` | Your Telegram Bot API key for notifications. | |
| `TELEGRAM_USER_ID` | Your Telegram User/Chat ID. | |

---

## 🏃‍♂️ Running with Docker

1.  **Start the Service**:
    -   Build and run the container in the background.
        ```bash
        docker compose -f compose.yaml up --build -d
        ```

2.  **Check the Logs**:
    -   See the application's real-time progress.
        ```bash
        docker compose -f compose.yaml logs -f
        ```
    -   *Detailed JSON logs will appear in the `./log` directory on your machine.*

3.  **Stopping the Service**:
    -   To stop the application, run:
        ```bash
        docker compose -f compose.yaml down
        ```
