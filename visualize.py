"""
Visualize data collected by experiment
"""
import matplotlib as mpl
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from collections import defaultdict
from pprint import pprint
import seaborn as sns
import matplotlib.ticker as ticker
from scipy.special import lambertw
import math


RESULT_FOLDER = "results/10tps4000delta8m/"

def genCrossTx(data):
    # Count numer of transactions grouped by crosstxes
    crosstxes = defaultdict(int)
    for _, tx in data.iterrows():
        crosstxes[int(tx['crosstxes'])] += 1 
    pprint(crosstxes)

    fig, ax = plt.subplots(nrows=1, ncols=1)

    # ax.spines['right'].set_visible(False)
    # ax.spines['top'].set_visible(False)

    ax.set_xlabel('Amount of cross committee transactions', labelpad=10)
    ax.set_ylabel('Number of transactions', labelpad=10)

    plt.setp(ax, xticks=range(len(crosstxes)), xticklabels=list(crosstxes.keys()))

    bars = ax.bar(range(len(crosstxes)), list(crosstxes.values()))
    # ax.set_xticks(range(len(crosstxes)), list(crosstxes.keys()))
    
    _width = [p.get_width() for p in ax.patches]
    width = _width[0]
    # for bar in bars:
    #     yval = bar.get_height()
    #     ax.text(bar.get_x() + width/3, yval + 5, yval)

    for rect in bars:
        height = rect.get_height()
        plt.text(rect.get_x() + rect.get_width()/2.0, height + 5, '%d' % int(height), ha='center', va='bottom')

    plt.show()

def genTx():
    file = RESULT_FOLDER + "tx.csv"
    data = pd.read_csv(file, header=None, names=["timestamp", "seconds", "crosstxes"])
    
    fig, ax = plt.subplots(nrows=1, ncols=3, figsize=(10,10))
    fig.tight_layout()
    
    for i in range(3):
        ax[i].set_xlabel("User-perceived latency in seconds", labelpad=10)
        ax[i].set_ylabel('Number of transactions', labelpad=10)

        x = data[data["crosstxes"] == i]['seconds']

        print("Mean: ", i, x.mean())
        print("Median: ", i, x.median())

        """
            Mean:  0 38.03530718232044
            Median:  0 36.3365
            Mean:  1 109.6901478424015
            Median:  1 108.02195
            Mean:  2 111.815837544484
            Median:  2 110.08000000000001
        """

        ax[i].hist(x, bins = 25, edgecolor = 'black')
        if i == 0:
            
            ax[i].set_yticks([i for i in range(20) if i % 3==0])
        ax[i].set_aspect(1./ax[i].get_data_ratio())

    plt.show()

def poc():
    file = RESULT_FOLDER + "pocadd.csv"
    pocadd = pd.read_csv(file, header=None, names=["timestamp", "nanoseconds"])

    file = RESULT_FOLDER + "pocverify.csv"
    pocverify = pd.read_csv(file, header=None, names=["timestamp", "nanoseconds"])

    fig, ax = plt.subplots(nrows=1, ncols=2, figsize=(10,10))
    fig.tight_layout()
    x1 = pocadd["nanoseconds"]
    x1 = x1/100000
    x2 = pocverify["nanoseconds"]
    x2 = x2/100000
    print("Mean 1", x1.mean())
    print("median 1", x1.median())
    print("minmax 1", min(x1), max(x1))
    print("mean 2: ", x2.mean())
    print("median 2:, ", x2.median())
    print("minmax 2", min(x1), max(x2))

    """

        Mean 1 2.6297844678750897
        median 1 2.08049
        minmax 1 0.65811 314.21649
        mean 2:  27.402288060792618
        median 2:,  20.8931
        minmax 2 0.65811 535.10623

    """

    x1 = pocadd[pocadd["nanoseconds"] < 10000000]["nanoseconds"] / 100000

    ax[0].set_xlabel("Milliseconds to add a PoC", labelpad=10)
    ax[0].set_ylabel('Proof of Consensuses', labelpad=10)

    ax[1].set_xlabel("Milliseconds to verify a PoC", labelpad=10)
    ax[1].set_ylabel('Proof of Consensuses', labelpad=10)

    # print("Mean: ", i, x.mean())
    # print("Median: ", i, x.median())

    ax[0].hist(x1, bins = 1000, edgecolor = 'blue')
    ax[1].hist(x2, bins = 900, edgecolor = 'blue')

    # ax[0].set_xticks(np.arange(min(x1), 7, 1))
    # ax[1].set_xticks(np.arange(min(x2), 150, 20))
    ax[0].xaxis.set_major_locator(ticker.MultipleLocator(1))
    ax[1].xaxis.set_minor_locator(ticker.MultipleLocator(10))

    ax[0].set_xlim(0, 7)
    ax[1].set_xlim(0, 50)


    ax[0].set_aspect(1./ax[0].get_data_ratio())
    ax[1].set_aspect(1./ax[1].get_data_ratio())

    plt.show()
    

def ida():
    file = RESULT_FOLDER + "ida.csv"
    data = pd.read_csv(file, header=None)

    # avg time for every reconstruction over time
    # time to completion bar

    fig, ax = plt.subplots(nrows=1, ncols=2, figsize=(10,10))
    fig.tight_layout()

    ax[0].set_ylabel("Average seconds to recover IDA message in a committee", labelpad=10)
    ax[0].set_xlabel('IDA message number', labelpad=15)

    ax[1].set_ylabel("Amount of IDA messages", labelpad=15)
    ax[1].set_xlabel('Seconds to successfully recover IDA message', labelpad=10)

    avgtime = list()
    timetocompletion = list()
    for _, row in data.iterrows():
        average = (row[2:] - row[1]).mean()
        average = np.abs(average)
        avgtime.append(average)

        tim = row[2:] - row[1]
        for t in tim:
            if np.isnan(t):
                continue
            t = np.abs(t)
            timetocompletion.append(t)

    print(timetocompletion)
    print("Average time: ", np.mean(avgtime))
    print("median time: ", np.median(avgtime))

    print("Avg timetocompletion", np.mean(timetocompletion))
    print("median timeto", np.median(timetocompletion))


    ax[0].plot(avgtime, linewidth=0.2)
    ax[0].set_xlim(xmin=0.0)
    ax[0].set_ylim(ymin=0.0)

    print(int(max(timetocompletion)))
    ax[1].hist(timetocompletion, bins = int(max(timetocompletion)), edgecolor = 'black')
    # ax[1].set_xticks([i for i in range(55) if i % 2 == 0])
    ax[1].xaxis.set_major_locator(ticker.MultipleLocator(4))

    ax[1].set_xlim(xmin=0.0)

    ax[0].set_aspect(1./ax[0].get_data_ratio())
    ax[1].set_aspect(1./ax[1].get_data_ratio())

    plt.show()


def routing():
    file = RESULT_FOLDER + "routing.csv"
    data = pd.read_csv(file, header=None, names=["timestamp", "start", "end"])

    # avg time for every reconstruction over time
    # time to completion bar

    fig, ax = plt.subplots(nrows=1, ncols=2, figsize=(10,10))
    fig.tight_layout()

    ax[0].set_ylabel("Average seconds to recover IDA message in a committee", labelpad=10)
    ax[0].set_xlabel('IDA message number', labelpad=15)

    # ax[1].set_ylabel("Amount of IDA messages", labelpad=15)
    # ax[1].set_xlabel('Seconds to successfully recover IDA message', labelpad=10)

    difference = data[data["start"] != 0]["end"] - data[data["start"] != 0]["start"]  

    for _, dif in difference.iteritems():
        if dif != 0:
            print(dif)

    """
    avgtime = list()
    timetocompletion = list()
    for _, row in data.iterrows():
        average = (row[2:] - row[1]).median()
        average = np.abs(average)
        avgtime.append(average)

        tim = row[2:] - row[1]
        for t in tim:
            if t == np.NaN:
                continue
            t = np.abs(t)
            timetocompletion.append(t)

    ax[0].plot(avgtime, linewidth=0.2)
    ax[0].set_xlim(xmin=0.0)
    ax[0].set_ylim(ymin=0.0)

    print(int(max(timetocompletion)))
    ax[1].hist(timetocompletion, bins = int(max(timetocompletion)), edgecolor = 'black')
    # ax[1].set_xticks([i for i in range(55) if i % 2 == 0])
    ax[1].xaxis.set_major_locator(ticker.MultipleLocator(4))

    ax[1].set_xlim(xmin=0.0)

    ax[0].set_aspect(1./ax[0].get_data_ratio())
    ax[1].set_aspect(1./ax[1].get_data_ratio())
    """

    plt.show()

def emulatepoc():
    # size of a PoC with m = 8 -> 250 and 2048kb / 250 bytes

    t = 250
    B = 2048000

    sizes = list()
    numerOfTx = list()
    f = 1/2

    # exponantial was too large to handle, so we use data from wolfram alpha instead 
    # https://www.wolframalpha.com/input/?i=%28-t+log%282%29+%2B+ProductLog%282%5E%28165+%2B+96+f+m+%2B+t%29+B+log%282%29%29%29%2Flog%282%29+where+B%3D2048000%2C+t%3D250%2C+f%3D0.5%2C+m%3Drange%288%2C250%29
    # range(8,250)
    data = [560.303, 608.221, 656.142, 704.068, 751.997, 799.929707257238, 847.865298944260, 895.803640173724, 943.744505906700, 991.687697639423, 1039.633039389829, 1087.580374415188, 1135.529562506628, 1183.480477743036, 1231.433006613803, 1279.387046440053, 1327.342504039157, 1375.299294588899, 1423.257340656514, 1471.216571364693, 1519.176921672008, 1567.138331749425, 1615.100746437905, 1663.064114774765, 1711.028389578591, 1758.993527084241, 1806.959486620832, 1854.926230326806, 1902.893722897046, 1950.861931357812, 1998.830824865922, 2046.800374529088, 2094.770553244803, 2142.741335555519, 2190.712697518179, 2238.684616586441, 2286.657071504122, 2334.630042208615, 2382.603509743181, 2430.577456177144, 2478.551864533165, 2526.526718720843, 2574.502003475994, 2622.477704305044, 2670.453807434023, 2718.430299761702, 2766.407168816491, 2814.384402716736, 2862.361990134084, 2910.339920259667, 2958.318182772813, 3006.296767812085, 3054.275665948440, 3102.254868160315, 3150.234365810478, 3198.214150624514, 3246.194214670779, 3294.174550341730, 3342.155150336506, 3390.136007644661, 3438.117115530970, 3486.098467521202, 3534.080057388813, 3582.061879142472, 3630.043927014359, 3678.026195449187, 3726.008679093886, 3773.991372787911, 3821.974271554112, 3869.957370590153, 3917.940665260422, 3965.924151088393, 4013.907823749440, 4061.891679064034, 4109.875712991329, 4157.859921623094, 4205.844301177969, 4253.828847996036, 4301.813558533675, 4349.798429358686, 4397.783457145668, 4445.768638671634, 4493.753970811848, 4541.739450535872, 4589.725074903809, 4637.710841062737, 4685.696746243302, 4733.682787756497, 4781.668962990573, 4829.655269408119, 4877.641704543257, 4925.628265998987, 4973.614951444638, 5021.601758613451, 5069.588685300254, 5117.575729359263, 5165.562888701957, 5213.550161295066, 5261.537545158637, 5309.525038364185, 5357.512639032927, 5405.500345334087, 5453.488155483274, 5501.476067740930, 5549.464080410838, 5597.452191838698, 5645.440400410755, 5693.428704552481, 5741.417102727323, 5789.405593435483, 5837.394175212760, 5885.382846629428, 5933.371606289170, 5981.360452828038, 6029.349384913460, 6077.338401243290, 6125.327500544887, 6173.316681574227, 6221.305943115057, 6269.295283978068, 6317.284703000112, 6365.274199043434, 6413.263770994946, 6461.253417765508, 6509.243138289257, 6557.232931522940, 6605.222796445286, 6653.212732056387, 6701.202737377110, 6749.192811448524, 6797.182953331347, 6845.173162105416, 6893.163436869169, 6941.153776739149, 6989.144180849518, 7037.134648351597, 7085.125178413415, 7133.115770219268, 7181.106422969304, 7229.097135879113, 7277.087908179329, 7325.078739115250, 7373.069627946472, 7421.060573946522, 7469.051576402516, 7517.042634614821, 7565.033747896728, 7613.024915574136, 7661.016136985247, 7709.007411480265, 7756.998738421110, 7804.990117181136, 7852.981547144865, 7900.973027707716, 7948.964558275752, 7996.956138265433, 8044.947767103373, 8092.939444226107, 8140.931169079863, 8188.922941120340, 8236.914759812496, 8284.906624630334, 8332.898535056707, 8380.890490583113, 8428.882490709508, 8476.874534944116, 8524.866622803253, 8572.858753811144, 8620.850927499757, 8668.843143408633, 8716.835401084723, 8764.827700082231, 8812.82003996246, 8860.81242029367, 8908.80484065091, 8956.79730061590, 9004.78979977687, 9052.78233772846, 9100.77491407154, 9148.76752841313, 9196.76018036624, 9244.75286954976, 9292.74559558835, 9340.73835811233, 9388.73115675752, 9436.72399116520, 9484.71686098194, 9532.70976585954, 9580.70270545490, 9628.69567942996, 9676.68868745152, 9724.68172919125, 9772.67480432552, 9820.66791253536, 9868.66105350631, 9916.65422692841, 9964.64743249606, 10012.64066990797, 10060.63393886704, 10108.62723908033, 10156.62057025896, 10204.61393211803, 10252.60732437655, 10300.60074675739, 10348.59419898718, 10396.58768079624, 10444.58119191856, 10492.57473209167, 10540.56830105664, 10588.56189855796, 10636.55552434353, 10684.54917816455, 10732.54285977550, 10780.53656893408, 10828.53030540114, 10876.52406894062, 10924.51785931952, 10972.51167630783, 11020.50551967848, 11068.49938920730, 11116.49328467297, 11164.48720585695, 11212.48115254347, 11260.47512451946, 11308.46912157448, 11356.46314350076, 11404.45719009304, 11452.45126114865, 11500.44535646735, 11548.43947585140, 11596.43361910544, 11644.42778603647, 11692.42197645385, 11740.41619016921, 11788.41042699644, 11836.40468675167, 11884.39896925320, 11932.39327432147, 11980.38760177906, 12028.38195145063, 12076.37632316287, 12124.37071674452, 12172.36513202629]

    for i, poc in enumerate(data):
        print(i)
        number_of_tx = math.floor(B/(t + poc))
        print("numtx", number_of_tx)
        numerOfTx.append(number_of_tx)

    # l = lambertw(118571099379011784113736688648896417641748464297615937576404566024103044751294464000 * np.log(2))/np.log(2) - 250
    # print(l)

    print("8", data[0])
    print("125", data[125-8])
    print("250", data[-1])

    print("tt8", numerOfTx[0])
    print("tt125", numerOfTx[125-8])
    print("tt250", numerOfTx[-1])

    """
    fig, ax = plt.subplots(nrows=1, ncols=2, figsize=(10,10))
    fig.tight_layout()

    ax[0].set_ylabel("Size of PoC", labelpad=10)
    ax[0].set_xlabel('Committee size', labelpad=15)

    ax[1].set_ylabel("Number of transactions in a 2048kb block", labelpad=15)
    ax[1].set_xlabel('Committee size', labelpad=10)

    ax[0].plot(range(8,251), data)
    # ax[0].set_xlim(xmin=0.0)
    # ax[0].set_ylim(ymin=0.0)

    # ax[1].hist(timetocompletion, bins = int(max(timetocompletion)), edgecolor = 'black')
    ax[1].plot(range(8,251), numerOfTx)


    #ax[1].xaxis.set_major_locator(ticker.MultipleLocator(4))

    #ax[1].set_xlim(xmin=0.0)

    ax[0].set_aspect(1./ax[0].get_data_ratio())
    ax[1].set_aspect(1./ax[1].get_data_ratio())

    plt.show()
    """


def routing():
    file = RESULT_FOLDER + "routing.csv"
    data = pd.read_csv(file, header=None, names=["timestamp", "start", "end"])

    # avg time for every reconstruction over time
    # time to completion bar

    fig, ax = plt.subplots(nrows=1, ncols=2, figsize=(10,10))
    fig.tight_layout()

    ax[0].set_ylabel("Average seconds to recover IDA message in a committee", labelpad=10)
    ax[0].set_xlabel('IDA message number', labelpad=15)

    # ax[1].set_ylabel("Amount of IDA messages", labelpad=15)
    # ax[1].set_xlabel('Seconds to successfully recover IDA message', labelpad=10)

    difference = data[data["start"] != 0]["end"] - data[data["start"] != 0]["start"]  

    for _, dif in difference.iteritems():
        if dif != 0:
            print(dif)


    plt.show()




def main():

    mpl.rcParams['axes.labelsize'] = 22
    plt.rcParams['font.size'] = 18
    plt.rcParams['axes.linewidth'] = 2

    #genTx()
    #poc()
 
    #ida(
    #routing()

    emulatepoc()


if __name__ == "__main__":
    main()