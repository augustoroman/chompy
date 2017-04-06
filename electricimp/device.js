// Report our MAC:
server.log("Starting up:");
server.log("Mac address: " + imp.getmacaddress());
server.log("SSID: " + imp.getssid());
server.log("Wifi signal strength: " + imp.rssi());

// Snack Dispenser
imp.setpowersave(true);

//Configure Pin
motor <- hardware.pin9;
motor.configure(DIGITAL_OUT);
motor.write(0);

agent.on("dispense", function(seconds) {
    server.log("Imp Dispensing: " + seconds + " seconds");
    motor.write(1);
    imp.wakeup(seconds, function(){
        motor.write(0);
    });
});
