import React from 'react';
import styles from '../styles'
import sad_img from '../assets/images/sad.gif'

export const renderDefaultPage = () => {
    return (
        <div className={`bg-auto  ${styles.boxWidth}`}>    
            <div className={`bg-gradient-to-t bg-slate-900 mb-4 ${styles.flexCenter}` }>
                <div className={`text-4xl mt-20  text-red-700 font-semibold`}>
                    DATA NOT FOUND
                </div>
            </div>
            <p className={`${styles.flexCenter} mb-4 text-3xl font-semibold text-indigo-600`}>404</p>
            <div className={`${styles.flexCenter} mb-20`}>
                <img src={sad_img} alt="Loading Animation"/>
            </div>
            <div className={`mb-64 ${styles.flexCenter}`}>
                <a href="/" className={`rounded-md bg-indigo-600 px-3.5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600  `}>Go back home</a>
            </div>
        </div>
    );
  };