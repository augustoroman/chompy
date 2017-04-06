const html = @"<!DOCTYPE html>
  <html>
  <head><title>Chompy</title></head>
  <body>
    Hungry?  Want some chompy credits?  Check in some code!
  </body>
</html>";

http.onrequest(function(request, res){
  try {
    if (request.path == "/status") {
      status(res);
    } else if (request.path == "/dispense") {
      dispense(request, res);
    } else if (request.path == "/") {
      res.send(200, html);
    } else {
      res.send(404, "Not found");
    }
  } catch (error) {
    res.send(500, error);
  }
});

function status(res) {
  res.send(200, "{\"online\":"+device.isconnected()+"}");
  return;
}

function dispense(request, res) {
  if(!device.isconnected()) {
    res.send(503, "device not connected");
    return;
  }

  local amount = 0.5;
  if ("amount" in request.query) {
    amount = request.query.amount.tofloat();
  }

  server.log("Agent: Dispensing for Chompy.");
  device.send("dispense", amount);

  res.send(200, "");
}
