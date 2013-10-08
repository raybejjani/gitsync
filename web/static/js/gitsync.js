$(function() {
    var ws = new WebSocket('ws://localhost:12345/events');
    ws.onmessage = function (evt) {
        var gitChange = JSON.parse(evt.data);
        console.log(JSON.stringify(gitChange));
        var user = gitChange['User'];
        var refName = gitChange['RefName'];
        var checkedOut = gitChange['CheckedOut'];
        if (checkedOut) {
            $('<div class="status">' + user + ' modified ' + 
              refName + '</div>')
                .hide()
                .appendTo('.container')
                .fadeIn(1000);
        }
    };
});