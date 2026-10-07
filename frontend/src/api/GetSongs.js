import { useState, useEffect } from "react";

export function GetSongs(url,start,end) {

    const [list, setData] = useState([]);
    console.log("GetSongs: " +url+ start + end)

    const time = {start:start,end:end}

    useEffect(() => {
        async function getData(start,end) {
            let response;
            const headers = { 
                'Content-Type': 'application/json',
            };
            if(start == null || end == null){
                response = await fetch(`${url}`, {
                    headers: headers,
                    method: 'GET',
                });
            }else{
                response = await fetch(`${url}?start_date=${encodeURIComponent(time.start)}&end_date=${encodeURIComponent(time.end)}`, { 
                    headers: headers,
                    method: 'GET',
                });
            }

            const data = await response.json();
            setData(data["message"]);
        }
        getData();
    }, []);
    return list;
}
