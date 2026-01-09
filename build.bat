@echo off
REM Build mdview and copy to C:\Tools

echo Building mdview...
go build -o mdview.exe .
if errorlevel 1 (
    echo Build failed!
    exit /b 1
)

echo Copying to C:\Tools...
copy /Y mdview.exe C:\Tools\mdview.exe

echo Done!
mdview.exe --version
