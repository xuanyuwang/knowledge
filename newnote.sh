#!/bin/zsh

# --- Determine Date ---
if [ -n "$1" ]; then
    DATE="$1"
else
    DATE=$(date +"%Y-%m-%d")
fi

# Validate format YYYY-MM-DD
if ! [[ "$DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
    echo "Error: date must be in format YYYY-MM-DD"
    exit 1
fi

# Paths
BASE_DIR="daily_notes"
DIR="$BASE_DIR/$DATE"
FILE="$DIR/notes.md"
TEMPLATE="templates/daily.md"

# Ensure template exists
if [ ! -f "$TEMPLATE" ]; then
    echo "Error: missing template at $TEMPLATE"
    exit 1
fi

# --- Create today's directory structure ---
mkdir -p "$DIR/images"

# --- Create notes.md if not exists ---
if [ ! -f "$FILE" ]; then
    # Inject title at the top, then append the template
    {
        echo "# Daily Engineering Notes – $DATE"
        echo ""
        cat "$TEMPLATE"
    } > "$FILE"

    echo "Created $FILE"
else
    echo "$FILE already exists"
fi
