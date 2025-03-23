import { spawn } from "child_process";
import { paths } from "../config";

export default function runScript(
  scriptName,
  args = [],
  onData = null,
  onError = null
) {
  return new Promise((resolve, reject) => {
    if (!paths.pythonExecutable) {
      reject(new Error("Python-Executable nicht gefunden!"));
      return;
    }

    const scriptPath = paths[scriptName];
    if (!scriptPath) {
      reject(new Error(`Ungültiger Skriptname: ${scriptName}`));
      return;
    }

    console.log("Starte Python-Skript:", scriptPath, "mit Argumenten:", args);
    const pythonProcess = spawn(paths.pythonExecutable, [scriptPath, ...args]);

    let outputData = "";
    let errorData = "";

    if (onData) {
      pythonProcess.stdout.on("data", (data) => onData(data));
    } else {
      pythonProcess.stdout.on("data", (data) => {
        outputData += data.toString().trim();
        console.log(`📄 stdout: ${outputData}`);
      });
    }
    if (onData) {
      pythonProcess.stderr.on("data", (data) => onError(data));
    } else {
      pythonProcess.stderr.on("data", (data) => {
        errorData += data.toString().trim();
        console.error(`⚠️ stderr: ${errorData}`);
      });
    }

    pythonProcess.on("close", (code) => {
      if (code !== 0) {
        reject(
          new Error(`Python exited with code ${code}, error: ${errorData}`)
        );
      } else {
        try {
          if (!outputData) {
            reject(new Error("Python output is empty"));
            return;
          }
          const result = JSON.parse(outputData);
          resolve(result);
        } catch (e) {
          reject(
            new Error(
              `Error parsing Python output: ${e.message}, raw output: ${outputData}`
            )
          );
        }
      }
    });

    pythonProcess.on("error", (err) => {
      reject(
        new Error(`Fehler beim Starten des Python-Prozesses: ${err.message}`)
      );
    });
  });
}
