const API_BASE = "http://localhost:8080";
let serverOnline = false;

// Check server status on page load
window.onload = function () {
  testHealth(true);

  // Upload type change handler
  document.querySelectorAll('input[name="uploadType"]').forEach((radio) => {
    radio.addEventListener("change", function () {
      const fileInput = document.getElementById("fileInput");
      const fileInputLabel = document.getElementById("fileInputLabel");

      if (this.value === "directory") {
        fileInput.setAttribute("webkitdirectory", "");
        fileInput.setAttribute("directory", "");
        fileInput.removeAttribute("multiple");
        fileInputLabel.textContent = "Choose directory:";
      } else {
        fileInput.removeAttribute("webkitdirectory");
        fileInput.removeAttribute("directory");
        fileInput.setAttribute("multiple", "");
        fileInputLabel.textContent = "Choose files:";
      }
      fileInput.value = ""; // Clear current selection
      document.getElementById("file-info").style.display = "none";
      document.getElementById("upload-btn").disabled = true;
    });
  });

  // File input change handler
  document.getElementById("fileInput").addEventListener("change", function (e) {
    const files = e.target.files;
    const uploadBtn = document.getElementById("upload-btn");
    const fileInfo = document.getElementById("file-info");
    const uploadType = document.querySelector(
      'input[name="uploadType"]:checked'
    ).value;

    if (files.length > 0) {
      let totalSize = 0;
      let fileTypes = {};

      for (let file of files) {
        totalSize += file.size;
        const ext = file.name.split(".").pop().toLowerCase();
        fileTypes[ext] = (fileTypes[ext] || 0) + 1;
      }

      const sizeInMB = (totalSize / (1024 * 1024)).toFixed(2);
      const typesList = Object.entries(fileTypes)
        .map(([ext, count]) => `${ext} (${count})`)
        .join(", ");

      let rootDir = "";
      if (uploadType === "directory" && files[0].webkitRelativePath) {
        rootDir = files[0].webkitRelativePath.split("/")[0];
      }

      fileInfo.innerHTML = `
        <strong>Selected:</strong> ${files.length} files (${uploadType})<br>
        <strong>Total Size:</strong> ${sizeInMB} MB<br>
        <strong>File Types:</strong> ${typesList}${
        rootDir ? `<br><strong>Root Directory:</strong> ${rootDir}` : ""
      }
      `;
      fileInfo.style.display = "block";
      uploadBtn.disabled = false;
    } else {
      fileInfo.style.display = "none";
      uploadBtn.disabled = true;
    }
  });
};

async function testHealth(silent = false) {
  const btn = document.getElementById("health-btn");
  const responseDiv = document.getElementById("health-response");
  const statusDiv = document.getElementById("server-status");

  if (!silent) {
    btn.disabled = true;
    btn.textContent = "Checking...";
  }

  try {
    const response = await fetch(`${API_BASE}/health`);

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();

    if (!silent) {
      responseDiv.innerHTML = `<span class="success">✅ Server is healthy!<br>Response: ${JSON.stringify(
        data,
        null,
        2
      )}</span>`;
      responseDiv.style.display = "block";
    }

    statusDiv.className = "status online";
    statusDiv.textContent = "Server Online";
    serverOnline = true;
  } catch (error) {
    if (!silent) {
      responseDiv.innerHTML = `<span class="error">❌ Failed to connect to server<br>Error: ${error.message}<br><br>Make sure the Go server is running on port 8080.</span>`;
      responseDiv.style.display = "block";
    }

    statusDiv.className = "status offline";
    statusDiv.textContent = "Server Offline";
    serverOnline = false;
  } finally {
    if (!silent) {
      btn.disabled = false;
      btn.textContent = "Check Server Health";
    }
  }
}

async function uploadFiles() {
  const fileInput = document.getElementById("fileInput");
  const files = fileInput.files;
  const btn = document.getElementById("upload-btn");
  const responseDiv = document.getElementById("upload-response");
  const progressDiv = document.getElementById("upload-progress");
  const progressBar = document.getElementById("progress-bar");

  if (files.length === 0) {
    alert("Please select files first");
    return;
  }

  btn.disabled = true;
  btn.textContent = "Uploading...";
  progressDiv.style.display = "block";
  progressBar.style.width = "0%";

  const formData = new FormData();

  // Add all files to FormData
  for (let file of files) {
    formData.append("files", file);
  }

  // Simulate progress during upload preparation
  progressBar.style.width = "25%";

  try {
    const xhr = new XMLHttpRequest();

    // Set up progress tracking
    xhr.upload.addEventListener("progress", function (e) {
      if (e.lengthComputable) {
        const percentComplete = (e.loaded / e.total) * 75 + 25; // 25-100%
        progressBar.style.width = percentComplete + "%";
      }
    });

    // Set up the response handler
    xhr.onload = function () {
      progressBar.style.width = "100%";

      try {
        const data = JSON.parse(xhr.responseText);
        const responseClass = data.success ? "success" : "error";
        const icon = data.success ? "✅" : "❌";

        responseDiv.innerHTML = `<span class="${responseClass}">${icon} Upload ${
          data.success ? "Successful" : "Failed"
        }!<br><pre>${JSON.stringify(data, null, 2)}</pre></span>`;
        responseDiv.style.display = "block";

        // Auto-fill directory ID fields for testing
        if (data.success && (data.uuid || data.directory_id)) {
          const uuid = data.uuid || data.directory_id;
          document.getElementById("directoryId").value = uuid;
          document.getElementById("readDirectoryId").value = uuid;
          document.getElementById("downloadDirectoryId").value = uuid;
          document.getElementById("zipDirectoryId").value = uuid;
        }
      } catch (parseError) {
        responseDiv.innerHTML = `<span class="error">❌ Upload failed - Invalid response format<br>Error: ${parseError.message}</span>`;
        responseDiv.style.display = "block";
      }

      btn.disabled = false;
      btn.textContent = "Upload Selected Files";
      setTimeout(() => {
        progressDiv.style.display = "none";
      }, 2000);
    };

    xhr.onerror = function () {
      responseDiv.innerHTML = `<span class="error">❌ Upload failed - Network error</span>`;
      responseDiv.style.display = "block";
      btn.disabled = false;
      btn.textContent = "Upload Selected Files";
      progressDiv.style.display = "none";
    };

    // Send the request
    xhr.open("POST", `${API_BASE}/upload`);
    xhr.send(formData);
  } catch (error) {
    responseDiv.innerHTML = `<span class="error">❌ Upload failed<br>Error: ${error.message}</span>`;
    responseDiv.style.display = "block";
    btn.disabled = false;
    btn.textContent = "Upload Selected Files";
    progressDiv.style.display = "none";
  }
}

async function listCodebases() {
  const btn = document.getElementById("list-btn");
  const responseDiv = document.getElementById("list-response");

  btn.disabled = true;
  btn.textContent = "Loading...";

  try {
    const response = await fetch(`${API_BASE}/codebases`);

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();
    const responseClass = data.success !== false ? "success" : "error";
    const icon = data.success !== false ? "✅" : "❌";

    responseDiv.innerHTML = `<span class="${responseClass}">${icon} Codebases Retrieved<br><pre>${JSON.stringify(
      data,
      null,
      2
    )}</pre></span>`;
    responseDiv.style.display = "block";
  } catch (error) {
    responseDiv.innerHTML = `<span class="error">❌ Failed to retrieve codebases<br>Error: ${error.message}</span>`;
    responseDiv.style.display = "block";
  } finally {
    btn.disabled = false;
    btn.textContent = "List All Codebases";
  }
}

async function getCodebaseDetails() {
  const directoryId = document.getElementById("directoryId").value.trim();
  const btn = document.getElementById("details-btn");
  const responseDiv = document.getElementById("details-response");

  if (!directoryId) {
    alert("Please enter a directory UUID");
    return;
  }

  btn.disabled = true;
  btn.textContent = "Loading...";

  try {
    const response = await fetch(`${API_BASE}/codebases/${directoryId}`);

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();
    const responseClass = data.success !== false ? "success" : "error";
    const icon = data.success !== false ? "✅" : "❌";

    responseDiv.innerHTML = `<span class="${responseClass}">${icon} Codebase Details<br><pre>${JSON.stringify(
      data,
      null,
      2
    )}</pre></span>`;
    responseDiv.style.display = "block";
  } catch (error) {
    responseDiv.innerHTML = `<span class="error">❌ Failed to retrieve codebase details<br>Error: ${error.message}</span>`;
    responseDiv.style.display = "block";
  } finally {
    btn.disabled = false;
    btn.textContent = "Get Codebase Details";
  }
}

async function readFileMetadata() {
  const directoryId = document.getElementById("readDirectoryId").value.trim();
  const filePath = document.getElementById("readFilePath").value.trim();
  const btn = document.getElementById("read-btn");
  const responseDiv = document.getElementById("read-response");

  if (!directoryId || !filePath) {
    alert("Please enter both directory UUID and file path");
    return;
  }

  btn.disabled = true;
  btn.textContent = "Reading...";

  try {
    const response = await fetch(
      `${API_BASE}/codebases/${directoryId}/content?file=${encodeURIComponent(
        filePath
      )}`
    );

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();
    const responseClass = data.success ? "success" : "error";
    const icon = data.success ? "✅" : "❌";

    // Truncate content if it's too long for display
    let displayData = { ...data };
    if (
      displayData.file &&
      displayData.file.content &&
      displayData.file.content.length > 1000
    ) {
      displayData.file.content =
        displayData.file.content.substring(0, 1000) +
        "\n... (content truncated for display)";
    }

    responseDiv.innerHTML = `<span class="${responseClass}">${icon} File Content<br><pre>${JSON.stringify(
      displayData,
      null,
      2
    )}</pre></span>`;
    responseDiv.style.display = "block";
  } catch (error) {
    responseDiv.innerHTML = `<span class="error">❌ Failed to read file content<br>Error: ${error.message}</span>`;
    responseDiv.style.display = "block";
  } finally {
    btn.disabled = false;
    btn.textContent = "Read File Metadata";
  }
}

async function downloadFile() {
  const directoryId = document
    .getElementById("downloadDirectoryId")
    .value.trim();
  const filePath = document.getElementById("downloadFilePath").value.trim();
  const btn = document.getElementById("download-btn");
  const responseDiv = document.getElementById("download-response");

  if (!directoryId || !filePath) {
    alert("Please enter both directory UUID and file path");
    return;
  }

  btn.disabled = true;
  btn.textContent = "Downloading...";

  try {
    const response = await fetch(
      `${API_BASE}/codebases/${directoryId}/download?file=${encodeURIComponent(
        filePath
      )}`
    );

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    // Create download link
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filePath.split("/").pop() || "downloaded_file"; // Get filename from path
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);

    responseDiv.innerHTML = `<span class="success">✅ File downloaded successfully!</span>`;
    responseDiv.style.display = "block";
  } catch (error) {
    responseDiv.innerHTML = `<span class="error">❌ Failed to download file<br>Error: ${error.message}</span>`;
    responseDiv.style.display = "block";
  } finally {
    btn.disabled = false;
    btn.textContent = "Download File";
  }
}

async function downloadZip() {
  const directoryId = document.getElementById("zipDirectoryId").value.trim();
  const btn = document.getElementById("zip-btn");
  const responseDiv = document.getElementById("zip-response");

  if (!directoryId) {
    alert("Please enter a directory UUID");
    return;
  }

  btn.disabled = true;
  btn.textContent = "Downloading ZIP...";

  try {
    const response = await fetch(`${API_BASE}/codebases/${directoryId}/zip`);

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    // Create download link
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `codebase-${directoryId}.zip`;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);

    responseDiv.innerHTML = `<span class="success">✅ ZIP file downloaded successfully!</span>`;
    responseDiv.style.display = "block";
  } catch (error) {
    responseDiv.innerHTML = `<span class="error">❌ Failed to download ZIP<br>Error: ${error.message}</span>`;
    responseDiv.style.display = "block";
  } finally {
    btn.disabled = false;
    btn.textContent = "Download as ZIP";
  }
}
