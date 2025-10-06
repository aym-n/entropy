# entropy 🌀: ai powered file organiztion

**Entropy** is a automated file organizer written in **Go**. It watches a designated folder (the "entropy" folder) and automatically moves new files into relevant subdirectories based on either **user-defined regular expressions** or intelligent **suggestions from the Google Gemini AI**. It is designed to reduce file clutter with minimal configuration.
-----

## ⚙️ Configuration (`rules.yaml`)

The sorter's behavior is controlled entirely by the `rules.yaml` file.

```yaml
options:
  preserve_structure: false # If true, the AI will ONLY suggest folders that already exist.
  knowledge_base: "knowledge.md" # Path to an optional file to give the AI context.

ignore:
  os_defaults: true 
  files:
    - ".DS_Store"
    - "Thumbs.db"
    - "desktop.ini"
  extensions:
    - ".log"
    - ".tmp"
  folders:
    - "node_modules"
    - "tmp"

rules:
  # Rule 1: Regex matches "invoice" anywhere and ends with ".pdf"
  - pattern: ".*invoice.*\\.pdf$"
    target: "Documents/Finance/Invoices"

  # Rule 2: Regex matches "resume" anywhere and ends with ".pdf"
  - pattern: ".*resume.*\\.pdf$"
    target: "Documents/Resumes"

gpt:
  enabled: true
  api_key: "AIzaSy..." # REPLACE with your actual Gemini API key!
  model: "gemini-2.0-flash-lite" # The model used for AI-powered suggestions
  instructions: |
    You are a file organization assistant. Given filename and MIME type,
    suggest a folder path. Respond only with the folder path.
```

### 🧠 Knowledge Base (`knowledge.md`)

The file specified in `options.knowledge_base` is loaded and appended to the AI's prompt. This allows you to provide crucial context to the model, improving its sorting accuracy.

**Example `knowledge.md` content:**

```markdown
Project-related files should go into "Work/Current/Project-X".
Personal photos from the last year go to "Archive/Photos/2025".
Any file related to 'go' or 'golang' should be placed in "Development/Go".
```

---

## 🚀 Getting Started

### Prerequisites

  * **Go:** A working Go environment (Go 1.18+ recommended).
  * **Gemini API Key:** An API key from Google AI Studio (required if `gpt.enabled` is set to `true` in `rules.yaml`).

### Installation

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/aym-n/entropy
    cd entropy
    ```

2.  **Run the application:**
    Since this is a single `main` package, you can run it directly:

    ```bash
    go run .
    ```

    *(For production use, you should build the executable: `go build . && ./entropy`)*

### Project Setup

The application automatically creates an `entropy` folder in the working directory and expects a configuration file named `rules.yaml`.

**Required Files:**

  * `main.go`
  * `go.mod` / `go.sum`
  * `rules.yaml` (Your provided configuration)
  * `knowledge.md` (Optional, referenced in `rules.yaml`)

-----

## 💡 Usage

1.  Start the program:

    ```bash
    go run .
    ```

    The terminal will show: `Watching 'entropy' folder...`

2.  **Drop a file** into the newly created `entropy` folder.

| File Dropped | Action | Log Output Example |
| :--- | :--- | :--- |
| `project_invoice_123.pdf` | **Rule-Based:** Matches Rule 1. | `Moved project_invoice_123.pdf → entropy/Documents/Finance/Invoices/project_invoice_123.pdf` |
| `AI_Project_Summary.docx` | **AI-Powered:** Does not match rules. | `AI suggested folder: Work/Projects/Reports` |
| `AI_Project_Summary.docx` | **Duplicate:** Same file dropped again. | `Moved AI_Project_Summary.docx → entropy/Work/Projects/Reports/AI_Project_Summary - 1.docx` |
