# Drone control service

- `alpha/`: Python backend (FastAPI + SQLite)
- `beta/`: Go backend (in progress)
- `web/`: React + Vite frontend

## 1. Start ArduPilot SITL

Make sure ArduPilot SITL is running first. The backend listens for MAVLink on UDP `127.0.0.1:14540`, so SITL has to send its traffic there with `--out`.

### Install (Ubuntu)

    sudo apt-get update && sudo apt-get install -y git
    git clone --recurse-submodules https://github.com/ArduPilot/ardupilot.git
    cd ardupilot
    Tools/environment_install/install-prereqs-ubuntu.sh -y
    . ~/.profile

Log out and back in (or open a new terminal) so `sim_vehicle.py` is on your `PATH`.

### Run 1 drone

    mkdir -p ~/sitl-runs && cd ~/sitl-runs
    sim_vehicle.py -v ArduCopter --console --map --out udp:127.0.0.1:14540

The first run builds SITL, which takes a few minutes. Add `-w` to wipe saved parameters back to defaults.

### Run 3 drones

    mkdir -p ~/sitl-runs && cd ~/sitl-runs
    sim_vehicle.py -v ArduCopter --count 3 --auto-sysid --auto-offset-line 90,10 --console --map --out udp:127.0.0.1:14540

`--auto-sysid` gives the drones system IDs 1, 2 and 3 so the backend can tell them apart. `--auto-offset-line 90,10` spawns them 10 m apart heading east. Each instance keeps its files in its own folder (`0/`, `1/`, `2/`), which is why these commands run from `~/sitl-runs`.

## 2. Start the backend and frontend

    docker compose --profile py up --build --watch   # Python backend + frontend
    docker compose --profile go up --build --watch   # Go backend + frontend

Either one builds the frontend and that backend, with hot reload on both.

Frontend: http://localhost:5173, API: http://localhost:8000.

## 3. QGroundControl (optional)

Run QGC alongside the frontend and backend to see the drones on a map until the frontend has better visuals. `sim_vehicle.py` also sends to UDP 14550 by default, which QGC picks up automatically.
