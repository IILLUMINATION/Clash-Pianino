import os

# Настройки
TARGET_DIR = "server"
ALLOWED_EXTENSIONS = {".go", ".proto", ".mod"}
IGNORE_DIRS = {".git", ".idea", "bin", "obj", "vendor"}

def collect_code():
    output = []
    
    for root, dirs, files in os.walk(TARGET_DIR):
        # Игнорируем ненужные папки
        dirs[:] = [d for d in dirs if d not in IGNORE_DIRS]
        
        for file in files:
            ext = os.path.splitext(file)[1]
            if ext in ALLOWED_EXTENSIONS:
                path = os.path.join(root, file)
                try:
                    with open(path, "r", encoding="utf-8") as f:
                        content = f.read()
                        output.append(f"--- FILE: {path} ---")
                        output.append(content)
                        output.append("\n")
                except Exception as e:
                    print(f"Error reading {path}: {e}")

    # Сохраняем в файл или выводим в консоль
    result = "\n".join(output)
    with open("project_context.txt", "w", encoding="utf-8") as f:
        f.write(result)
    print("Готово! Код сохранен в 'project_context.txt'")

if __name__ == "__main__":
    collect_code()