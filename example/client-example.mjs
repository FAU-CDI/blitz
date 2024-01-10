import {request} from "http"

// Example script to make a request via blitz with a reservation.
// Start the server with: 
// go run ./cmd/blitz -queue 1 -target https://example.com

/** make a request to localhost:8080/${path} with the given method and headers */
function makeRequest(path, method, headers) {
    return new Promise((rs, rj) => {
        const req = request({
            hostname: 'localhost', port: 8080, path: path,
            method: method,
            headers: headers, 
        }, (res) => {
            let body = '';
            res.on('data', (chunk) => body += chunk);
            res.on('end', () => {
                if (res.statusCode !== 200) {
                    rj("Invalid response code " + res.statusCode + " with body " + JSON.stringify(body));
                    return;
                }

                rs(body);
            })
        })
    
        req.on('error', (err) => {rj(err)});
        req.end();
    })
}

/** sleep for the number of milliseconds */
function sleep(milliseconds) {
    if(milliseconds <= 0) return Promise.resolve();

    return new Promise((rs, rj) => setTimeout(() => rs(), milliseconds))
}
 

// make a post request to /blitz/ to get the reservation
const reservation = await makeRequest('/blitz/', 'POST', undefined).then(JSON.parse.bind(JSON));
console.log("got reservation", reservation);

// wait for the reservation to be valid
await sleep(reservation['DelayInMilliseconds']);

// make a request with the reservation to the root url
const data = await makeRequest('/', 'GET', {
    'Host': 'example.com',
    'X-Blitz-Reservation': reservation['X-Blitz-Reservation'],
})
console.log("got response: " + data);
