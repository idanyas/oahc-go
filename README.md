# 🤖 OCI ARM Host Capacity

A lightweight, modern, and efficient Go application that tirelessly scans Oracle Cloud Infrastructure (OCI) for available **"Always Free" Ampere A1** compute capacity.

When capacity is found, it automatically provisions an instance based on your configuration and sends you a Telegram notification. This project is distributed as a **multi-architecture Docker image**, making setup incredibly simple.

---

### ✨ Features

-   🚀 **Zero-Build Setup**: Runs directly from a public Docker image. No need to install Go or clone the repository.
-   🌐 **Multi-Architecture**: The image runs natively on both `amd64` (Intel/AMD) and `arm64` (Apple Silicon, Raspberry Pi, OCI ARM) systems.
-   🤖 **Automated Provisioning**: Runs 24/7 and automatically creates an instance the moment capacity is available.
-   ⚙️ **Flexible Configuration**: Define your desired instance shape, OCPU count, memory, and boot volume.
-   🔔 **Telegram Notifications**: Get an instant alert on success with all the details of your new instance.
-   🧠 **Intelligent Backoff**: Automatically handles OCI API rate limits by waiting and retrying.

### 🤔 How It Works

1.  **Load Config**: Reads your OCI and instance settings from the `.env` file.
2.  **Check Existing**: Checks if you've already reached your maximum desired instances. If so, it exits.
3.  **Scan & Create**: It loops through the availability domains in your region, attempting to create an instance.
    -   *On "Out of Capacity"*: It logs the message and immediately tries the next domain.
    -   *On "Too Many Requests"*: It waits for a dynamically increasing period before trying again.
    -   *On Success*: It creates the instance, sends a Telegram notification, and exits gracefully.

---

## 🚀 Quick Start Guide

You only need **Docker** and **Docker Compose** installed.

1.  **Create a Project Directory:**
    ```bash
    mkdir oahc-finder
    cd oahc-finder
    ```

2.  **Download Configuration Files:**
    ```bash
    # Download the compose file
    wget https://raw.githubusercontent.com/idanyas/oahc-go/main/compose.yaml
    
    # Download the environment file example
    wget https://raw.githubusercontent.com/idanyas/oahc-go/main/.env.example -O .env
    ```

3.  **Complete the OCI Setup & Configure `.env`:**
    -   Follow the **Detailed Configuration Guide** below to get your OCI credentials and resource IDs.
    -   Place your OCI private key in this directory and name it `oci_api_key.pem`.
    -   Edit the `.env` file with all your details.

4.  **Run the Service:**
    ```bash
    docker compose up -d
    ```

5.  **Check the Logs:**
    ```bash
    docker compose logs -f
    ```

---

## ⚙️ Detailed Configuration Guide

Follow these steps to get all the necessary credentials and IDs to populate your `.env` file.

### ✅ Prerequisites

*   An [Oracle Cloud Infrastructure](https://cloud.oracle.com/) account.
*   [OCI CLI](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm) & [`jq`](https://stedolan.github.io/jq/download/) installed for finding resource IDs.

### Step 1: Get Primary Credentials from OCI

1.  **Navigate to API Keys**:
    -   In the OCI Console, go to **Profile ➡️ My Profile ➡️ API Keys**.

2.  **Add a New API Key**:
    -   Click **Add API Key** and select **Generate API Key Pair**.
    -   Click **Download Private Key** and save the file as `oci_api_key.pem` in your project folder.

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
    -   For `key_file`, you must provide the **full, absolute path** to the `oci_api_key.pem` file you created.
    -   *Example `key_file`: `/home/youruser/oahc-finder/oci_api_key.pem`*

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

1.  **Open and edit the `.env` file** in your project directory, filling in all the required values you've gathered from the previous steps.

| Variable | Description | Required |
| :--- | :--- | :---: |
| `OCI_USER_ID` | The `user` value from Step 1. | ✅ |
| `OCI_TENANCY_ID` | The `tenancy` value from Step 1. | ✅ |
| `OCI_KEY_FINGERPRINT`| The `fingerprint` value from Step 1. | ✅ |
| `OCI_REGION` | The `region` value from Step 1. | ✅ |
| `OCI_PRIVATE_KEY_FILENAME`| Path inside the container. The `compose.yaml` maps your local key to this path. **Should be `/app/oci_api_key.pem`**. | ✅ |
| `OCI_SUBNET_ID` | An OCID from Step 3. | ✅ |
| `OCI_IMAGE_ID` | An OCID from Step 3. | ✅ |
| `OCI_SSH_PUBLIC_KEY`| The **full content** of your public SSH key (`~/.ssh/id_rsa.pub`). | ✅ |
| `OCI_AVAILABILITY_DOMAIN` | Specific AD to try. *Leave empty to try all*. | |
| `TELEGRAM_BOT_API_KEY` | Your Telegram Bot API key for notifications. | |
| `TELEGRAM_USER_ID` | Your Telegram User/Chat ID. | |

---

## 🏃‍♂️ Running with Docker

Once you have completed the **Quick Start Guide**, you can use these commands to manage the service.

1.  **Start the Service**:
    -   Run the container in the background. If you want the latest version, run `docker compose pull` first.
        ```bash
        docker compose up -d
        ```

2.  **Check the Logs**:
    -   See the application's real-time progress.
        ```bash
        docker compose logs -f
        ```
    -   *Detailed JSON logs will appear in the `./logs` directory on your machine.*

3.  **Stopping the Service**:
    -   To stop the application, run:
        ```bash
        docker compose down
        ```