import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  // A number specifying the number of VUs to run concurrently.
  vus: 2,
  // A string specifying the total duration of the test run.
  duration: '150m',
};

// The function that defines VU logic.
//
// See https://grafana.com/docs/k6/latest/examples/get-started-with-k6/ to learn more
// about authoring k6 scripts.
//
export default function() {
    let address = 'http://192.168.1.36:54444';
    // let address = 'http://192.168.1.12:44444';
    http.get(address + '/fake/fsf');
    http.get(address + '/combination/1');
    http.get(address + '/direct/slow');
    http.get(address + '/direct/delayed');
    http.get(address + '/direct/drop');
    http.get(address + '/does_not_exist');
    sleep(1);
}
