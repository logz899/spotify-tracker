import React, { useState, useEffect } from 'react';
import styles from '../../styles';
import { NavBar } from '../../components/Navbar';
import { Tabs } from '../../components/Tabs';
import { SongsList } from '../../components/SongsList';
import { ArtistList } from '../../components/AritstList';
import { renderDefaultPage } from '../../components/DefaultPage';
import ScrollToTop from "react-scroll-to-top";

let d = new Date()
let startdd = `${d.getFullYear()}-${d.getMonth()}-${1}`
let enddd = `${d.getFullYear()}-${d.getMonth()+1}-1`

export const LastMonth = () => {

    const [songs, setSongs] = useState([]);
    const [userInfo, setUserInfo] = useState([]);
    const [artists, setArtists] = useState([]);
    const [start, setStart] = useState(`${d.getFullYear}-${d.getMonth-1}-${1}`);
    const [end, setEnd] = useState(`${d.getFullYear}-${d.getMonth}-${1}`);
    const [currentTab, setCurrentTab] = useState("Song");

    const handleTabChange = (newTab) => {
        console.log('newTab: ', newTab)
        setCurrentTab(newTab);
    };

    const dateTransform = (date) => {
        let year = date.getFullYear();
        let month = date.getMonth() + 1;
        let dt = date.getDate();
        let dateString = `${year}-${month}-${dt}`;
        return dateString;
    }


    const fetchDataApi = async (url, endpoint, date, method = 'GET') => {
        try {
            console.log(`${url}${endpoint}${date}`);
            const response = await fetch(`${url}${endpoint}${date}`, {
                method: method,
                headers: {
                    'Content-Type': 'application/json',
                },
            });
    
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return await response.json();
        } catch (error) {
            console.error('Error fetching data:', error);
            return null;
        }
    }

    const songsAndArtistsApi = async (url, endpoint, date) => {
        return await fetchDataApi(url, endpoint, date);
    };

    const selectspecificDate = (sdate,edate) => {
        sdate = new Date(sdate);
        edate = new Date(edate);
        const startDate = dateTransform(sdate);
        const endDate = dateTransform(edate);

        setStart(startDate);
        setEnd(endDate);

        let dateString = '';
        if(sdate && edate){
            dateString = `?start_date=${encodeURIComponent(startDate)}&end_date=${encodeURIComponent(endDate)}`
        }

        songsAndArtistsApi('http://localhost:8000/', 'songs', dateString)
        .then(data => {
            setSongs(data["message"]);
            setUserInfo(data["user_info"]);
        }).catch(error => {
            console.error(error);
        });

        songsAndArtistsApi('http://localhost:8000/', 'artists', dateString)
        .then(data => {
            setArtists(data["message"]);
        }).catch(error => {
            console.error(error);
        });
    }

    useEffect(() => {
        songsAndArtistsApi('http://localhost:8000/', 'songs', `?start_date=${encodeURIComponent(startdd)}&end_date=${encodeURIComponent(enddd)}`)
        .then(data => {
            setSongs(data["message"]);
            setUserInfo(data["user_info"]);
        }).catch(error => {
            console.error(error);
        });
        songsAndArtistsApi('http://localhost:8000/', 'artists', `?start_date=${encodeURIComponent(startdd)}&end_date=${encodeURIComponent(enddd)}`)
        .then(data => {
            setArtists(data["message"]);
        }).catch(error => {
            console.error(error);
        });
    }, []);

    return(
        <div className={`bg-black w-full overflow-hidden `}>
            <div className={`${styles.flexCenter}`}>
                <div className={`w-full bg-gradient-to-r bg-slate-900`}>
                    <NavBar data={userInfo} select={selectspecificDate}/>
                </div>
            </div>
            <div className='w-full bg-gradient-to-r bg-slate-900'>
                <Tabs currentTab={currentTab} onTabChange={handleTabChange} />  
            </div>
            <div className={`bg-gradient-to-t h-dvh  bg-slate-900 ${styles.paddingX} ${styles.flexStart}`}>
                <div className={`${styles.boxWidth} `}>
                    {songs.length === 0 && artists.length === 0 && renderDefaultPage()}
                    {currentTab === 'Song' && songs !== 0? (
                        <SongsList data={songs} />
                    ) : currentTab === 'Artist' && songs !== 0? (
                        <ArtistList data={artists} />
                    )  : (
                        renderDefaultPage()
                    )}
                </div>
                <ScrollToTop smooth />
            </div>
        </div>
    );
}