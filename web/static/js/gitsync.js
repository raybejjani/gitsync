function currentTimestamp() {
    function pad(n){return n < 10 ? '0' + n : n;}
    var date = new Date();
    return pad(date.getHours()) + ':'
        + pad(date.getMinutes());
}

$(function() {
    var ws = new WebSocket('ws://localhost:12345/events');
    ws.onmessage = function (evt) {
        var gitChange = JSON.parse(evt.data);
        var user = gitChange['User'];
        var refName = gitChange['RefName'];
        var checkedOut = gitChange['CheckedOut'];
        var timestamp = currentTimestamp();
        if (checkedOut) {
            $('<div class="status">' + user + ' modified ' + 
              refName + ' <span class="timestamp">' + timestamp + '</span></div>')
                .hide()
                .appendTo('.container')
                .fadeIn(1000);
        }
    };
});